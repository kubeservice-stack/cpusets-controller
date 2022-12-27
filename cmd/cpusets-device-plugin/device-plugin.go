/*
Copyright 2022 The KubeService-Stack Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/kubeservice-stack/common/pkg/logger"
	"github.com/kubeservice-stack/cpusets-controller/pkg/client"
	"github.com/kubeservice-stack/cpusets-controller/pkg/config"
	"github.com/kubeservice-stack/cpusets-controller/pkg/topology"
	"github.com/kubeservice-stack/cpusets-controller/pkg/types"
	"golang.org/x/net/context"
	grpc "google.golang.org/grpc"
	"k8s.io/client-go/kubernetes"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	"k8s.io/kubernetes/pkg/kubelet/cm/cpuset"
)

var (
	resourceBaseName = "cmss.cn"
	cdms             []*cpuDeviceManager
	mainLogger       = logger.GetLogger("cmd/cpusets-device-plugin", "main")
)

type cpuDeviceManager struct {
	pool           types.Pool
	socketFile     string
	grpcServer     *grpc.Server
	sharedPoolCPUs string
	poolType       string
	nodeTopology   map[int]int
	htTopology     map[int]string
}

//TODO: PoC if cpuset setting could be implemented in this hook? cpuset cgroup of the container should already exist at this point (kinda)
//The DeviceIDs could be used to determine which container has them, once we have a container name parsed out from the allocation backend we could manipulate its cpuset before it is even started
//Long shot, but if it works both cpusetter and process starter would become unnecessary
func (cdm *cpuDeviceManager) PreStartContainer(ctx context.Context, psRqt *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}

func (cdm *cpuDeviceManager) Start() error {
	pluginEndpoint := filepath.Join(pluginapi.DevicePluginPath, cdm.socketFile)
	mainLogger.Info("Starting CPU Device Plugin server", logger.Any("endpoint", pluginEndpoint))
	lis, err := net.Listen("unix", pluginEndpoint)
	if err != nil {
		mainLogger.Error("Error. Starting CPU Device Plugin server failed", logger.Error(err))
	}
	cdm.grpcServer = grpc.NewServer()

	// Register all services
	pluginapi.RegisterDevicePluginServer(cdm.grpcServer, cdm)

	go cdm.grpcServer.Serve(lis)

	// Wait for server to start by launching a blocking connection
	conn, err := grpc.Dial(pluginEndpoint, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}),
	)
	if err != nil {
		mainLogger.Error("Error. Could not establish connection with gRPC server", logger.Error(err))
		return err
	}
	mainLogger.Info("CPU Device Plugin server started serving")
	conn.Close()
	return nil
}

func (cdm *cpuDeviceManager) cleanup() error {
	pluginEndpoint := filepath.Join(pluginapi.DevicePluginPath, cdm.socketFile)
	if err := os.Remove(pluginEndpoint); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (cdm *cpuDeviceManager) Stop() error {
	mainLogger.Info("CPU Device Plugin gRPC server..")
	if cdm.grpcServer == nil {
		return nil
	}
	cdm.grpcServer.Stop()
	cdm.grpcServer = nil
	return cdm.cleanup()
}

func (cdm *cpuDeviceManager) ListAndWatch(e *pluginapi.Empty, stream pluginapi.DevicePlugin_ListAndWatchServer) error {
	var updateNeeded = true
	for {
		if updateNeeded {
			resp := new(pluginapi.ListAndWatchResponse)
			if cdm.poolType == "shared" {
				nbrOfCPUs := cdm.pool.CPUset.Size()
				for i := 0; i < nbrOfCPUs*1000; i++ {
					cpuID := strconv.Itoa(i)
					resp.Devices = append(resp.Devices, &pluginapi.Device{ID: cpuID, Health: pluginapi.Healthy})
				}
			} else {
				for _, cpuID := range cdm.pool.CPUset.ToSlice() {
					exclusiveCore := pluginapi.Device{ID: strconv.Itoa(cpuID), Health: pluginapi.Healthy}
					if numaNode, exists := cdm.nodeTopology[cpuID]; exists {
						exclusiveCore.Topology = &pluginapi.TopologyInfo{Nodes: []*pluginapi.NUMANode{{ID: int64(numaNode)}}}
					}
					resp.Devices = append(resp.Devices, &exclusiveCore)
				}
			}
			if err := stream.Send(resp); err != nil {
				mainLogger.Error("Error. Cannot update device states", logger.Error(err))
				return err
			}
			updateNeeded = false
		}
		//TODO: When is update needed ?
		time.Sleep(5 * time.Second)
	}
	return nil

}

func (cdm *cpuDeviceManager) Allocate(ctx context.Context, rqt *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	resp := new(pluginapi.AllocateResponse)
	for _, container := range rqt.ContainerRequests {
		envmap := make(map[string]string)
		cpusAllocated, _ := cpuset.Parse("")
		for _, id := range container.DevicesIDs {
			tempSet, _ := cpuset.Parse(id)
			cpusAllocated = cpusAllocated.Union(tempSet)
		}
		if cdm.pool.HTPolicy == types.MultiThreadHTPolicy {
			cpusAllocated = topology.AddHTSiblingsToCPUSet(cpusAllocated, cdm.htTopology)
		}
		if cdm.poolType == "shared" {
			envmap["SHARED_CPUS"] = cdm.sharedPoolCPUs
		} else {
			envmap["EXCLUSIVE_CPUS"] = cpusAllocated.String()
		}
		containerResp := new(pluginapi.ContainerAllocateResponse)
		mainLogger.Info("CPUs allocated: " + cpusAllocated.String() + ": Num of CPUs %s" +
			strconv.Itoa(cpusAllocated.Size()))

		containerResp.Envs = envmap
		resp.ContainerResponses = append(resp.ContainerResponses, containerResp)
	}
	return resp, nil
}

func (cdm *cpuDeviceManager) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	dpOptions := pluginapi.DevicePluginOptions{
		PreStartRequired:                false,
		GetPreferredAllocationAvailable: false,
	}
	return &dpOptions, nil
}

func (cdm *cpuDeviceManager) Register(kubeletEndpoint, resourceName string) error {
	conn, err := grpc.Dial(kubeletEndpoint, grpc.WithInsecure(),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}))
	if err != nil {
		mainLogger.Error("CPU Device Plugin cannot connect to Kubelet service", logger.Error(err))
		return err
	}
	defer conn.Close()
	client := pluginapi.NewRegistrationClient(conn)

	request := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     cdm.socketFile,
		ResourceName: resourceName,
	}

	if _, err = client.Register(context.Background(), request); err != nil {
		mainLogger.Error("CPU Device Plugin cannot register to Kubelet service", logger.Error(err))
		return err
	}
	return nil
}

func (cdm *cpuDeviceManager) GetPreferredAllocation(context.Context, *pluginapi.PreferredAllocationRequest) (*pluginapi.PreferredAllocationResponse, error) {
	return &pluginapi.PreferredAllocationResponse{}, nil
}

func newCPUDeviceManager(poolName string, pool types.Pool, sharedCPUs string) *cpuDeviceManager {
	mainLogger.Info("Starting plugin for pool: " + poolName)
	return &cpuDeviceManager{
		pool:           pool,
		socketFile:     fmt.Sprintf("cpudp_%s.sock", poolName),
		sharedPoolCPUs: sharedCPUs,
		poolType:       types.DeterminePoolType(poolName),
		nodeTopology:   topology.GetNodeTopology(),
		htTopology:     topology.GetHTTopology(),
	}
}

func validatePools(poolConf types.PoolConfig) (string, error) {
	var sharedCPUs string
	var err error
	for poolName, pool := range poolConf.Pools {
		poolType := types.DeterminePoolType(poolName)
		if poolType == types.SharedPoolID {
			if sharedCPUs != "" {
				err = fmt.Errorf("Only one shared pool allowed")
				mainLogger.Error("Pool config error", logger.Any("poolConf", poolConf), logger.Error(err))
				break
			}
			sharedCPUs = pool.CPUset.String()
		}
	}
	return sharedCPUs, err
}

func createCDMs(poolConf types.PoolConfig, sharedCPUs string) error {
	var err error
	for poolName, pool := range poolConf.Pools {
		poolType := types.DeterminePoolType(poolName)
		//Deault or unrecognizable pools need not be made available to Device Manager as schedulable devices
		if poolType == types.DefaultPoolID {
			continue
		}
		cdm := newCPUDeviceManager(poolName, pool, sharedCPUs)
		cdms = append(cdms, cdm)
		if err := cdm.Start(); err != nil {
			mainLogger.Error("cpuDeviceManager.Start() failed", logger.Error(err))
			break
		}
		resourceName := resourceBaseName + "/" + poolName
		err := cdm.Register(path.Join(pluginapi.DevicePluginPath, "kubelet.sock"), resourceName)
		if err != nil {
			// Stop server
			cdm.grpcServer.Stop()
			mainLogger.Error("Register error", logger.Error(err))
			break
		}
		mainLogger.Info("CPU device plugin registered with the Kubelet")
	}
	return err
}

func createPluginsForPools(c kubernetes.Interface) error {
	files, err := filepath.Glob(filepath.Join(pluginapi.DevicePluginPath, config.FileMatch))
	if err != nil {
		mainLogger.Error("filepath glob error!", logger.Error(err))
	}
	for _, f := range files {
		if err := os.Remove(f); err != nil {
			mainLogger.Error("os.Remove error!", logger.Error(err))
		}
	}
	poolConf, err := types.DeterminePoolConfig(c, config.FileMatch, config.NodeName)
	if err != nil {
		mainLogger.Error("types.DeterminePoolConfig error!", logger.Error(err))
	}
	mainLogger.Info("Pool configuration", logger.Any("poolconf", poolConf))

	var sharedCPUs string
	sharedCPUs, err = validatePools(poolConf)
	if err != nil {
		return err
	}

	if err := createCDMs(poolConf, sharedCPUs); err != nil {
		for _, cdm := range cdms {
			cdm.Stop()
		}
	}
	return err
}

func main() {
	flag.Parse()
	watcher, _ := fsnotify.NewWatcher()
	watcher.Add(path.Join(pluginapi.DevicePluginPath, "kubelet.sock"))
	defer watcher.Close()

	// respond to syscalls for termination
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	_ = client.KubeClient()

	if err := createPluginsForPools(client.Clientset); err != nil {
		mainLogger.Error("Failed to start device plugin", logger.Error(err))
	}

	/* Monitor file changes for kubelet socket file and termination signals */
	for {
		select {
		case sig := <-sigCh:
			switch sig {
			case syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT:
				mainLogger.Info("Received signal, shutting down.", logger.Any("signal", sig))
				for _, cdm := range cdms {
					cdm.Stop()
				}
				return
			}
			mainLogger.Info("Received signal!", logger.Any("signal", sig))

		case event := <-watcher.Events:
			mainLogger.Info("Kubelet change event in pluginpath ", logger.Any("event", event))

			for _, cdm := range cdms {
				cdm.Stop()
			}
			cdms = nil
			if err := createPluginsForPools(client.Clientset); err != nil {
				panic("Failed to restart device plugin")
			}
		}
	}
}
