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
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kubeservice-stack/common/pkg/logger"
	"github.com/kubeservice-stack/cpusets-controller/pkg/client"
	"github.com/kubeservice-stack/cpusets-controller/pkg/config"
	"github.com/kubeservice-stack/cpusets-controller/pkg/controller"
	"github.com/kubeservice-stack/cpusets-controller/third_party/forked/nokia/CPU-Pooler/pkg/types"
)

const (
	//NumberOfWorkers controls how many asynch event handler threads are started in the CPUSetter controller
	NumberOfWorkers = 100
)

var (
	kubeConfig     string
	poolConfigPath string
	cpusetRoot     string
	mainLogger     = logger.GetLogger("cmd/cpusets-controller", "main")
)

func main() {
	flag.Parse()
	if poolConfigPath == "" || cpusetRoot == "" {
		log.Fatal("ERROR: Mandatory command-line arguments poolconfigs and cpusetroot were not provided!")
	}
	c, err := client.KubeConfigClientSet(kubeConfig)
	if err != nil {
		log.Fatal("ERROR: Could not initalize K8s client because of error:" + err.Error() + ", exiting!")
	}
	poolConf, err := types.DeterminePoolConfig(c, config.FileMatch, config.NodeName)
	if err != nil {
		log.Fatal("ERROR: Could not read CPU pool configuration files because: " + err.Error() + ", exiting!")
	}
	cc, err := controller.New(kubeConfig, poolConf, cpusetRoot)
	if err != nil {
		log.Fatal("ERROR: Could not initalize K8s client because of error: " + err.Error() + ", exiting!")
	}

	stopChannel := make(chan struct{})
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGINT, syscall.SIGTERM)
	log.Println("CPUSetter's Controller initalized successfully!")
	cc.Run(NumberOfWorkers, &stopChannel)
	select {
	case <-signalChannel:
		log.Println("Orchestrator initiated graceful shutdown, ending CPUSetter workers...(o_o)/")
		cc.Stop()
	}
}

func init() {
	flag.StringVar(&poolConfigPath, "poolconfigs", "", "Path to the pool configuration files. Mandatory parameter.")
	flag.StringVar(&cpusetRoot, "cpusetroot", "", "The root of the cgroupfs where Kubernetes creates the cpusets for the Pods . Mandatory parameter.")
	flag.StringVar(&kubeConfig, "kubeconfig", "", "Path to a kubeconfig. Optional parameter, only required if out-of-cluster.")
}
