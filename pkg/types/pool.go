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

package types

import (
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/kubeservice-stack/common/pkg/logger"
	"github.com/kubeservice-stack/cpusets-controller/pkg/client"
	"gopkg.in/yaml.v2"
	k8sclient "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/kubelet/cm/cpuset"
)

const (
	// SharedPoolID 是 CPU 池名称中的常量前缀. 表示 CPU 池是共享类型的
	SharedPoolID = "shared"
	// ExclusivePoolID 是 CPU 池名称中的常量前缀. 表示 CPU 池是独占类型
	ExclusivePoolID = "exclusive"
	// DefaultPoolID 是 CPU 池名称中的常量前缀. 表示 CPU 池是默认类型
	DefaultPoolID = "default"
	// SingleThreadHTPolicy 是 HT 策略池属性的单线程值的常量。设置该值时，只为独占请求分配物理线程
	SingleThreadHTPolicy = "singleThreaded"
	// MultiThreadHTPolicy 是 HT 策略池属性的多线程值的常量。设置此值时，所有兄弟一起分配用于独占请求
	MultiThreadHTPolicy = "multiThreaded"
)

var (
	//PoolConfigDir defines the pool configuration file location
	PoolConfigDir = "/etc/cpusets-pool"
)

// Pool defines cpupool
type Pool struct {
	CPUset   cpuset.CPUSet
	CPUStr   string `yaml:"cpus"`
	HTPolicy string `yaml:"hyperThreadingPolicy"`
}

// PoolConfig defines pool configuration for a node
type PoolConfig struct {
	Pools        map[string]Pool   `yaml:"pools"`
	NodeSelector map[string]string `yaml:"nodeSelector"`
}

//SelectPool returns the exact CPUSet belonging to either the exclusive, shared, or default pool of one PoolConfig object
//An empty CPUSet is returned in case the configuration does not contain the requested type
func (p *PoolConfig) SelectPoolConfig(prefix string) Pool {
	for poolName, pool := range p.Pools {
		if strings.HasPrefix(poolName, prefix) {
			return pool
		}
	}
	return Pool{}
}

// parsePoolConfigFile reads a pool configuration file
func parsePoolConfigFile(name string) (PoolConfig, error) {
	file, err := ioutil.ReadFile(name)
	if err != nil {
		typesLogger.Error(ErrNotReadPoolConfig.Error(), logger.Error(err), logger.Any("file", name))
		return PoolConfig{}, ErrNotReadPoolConfig
	}

	var poolConfig PoolConfig
	err = yaml.Unmarshal([]byte(file), &poolConfig)
	if err != nil {
		typesLogger.Error(ErrNotParsePoolConfig.Error(), logger.Error(err))
		return PoolConfig{}, ErrNotParsePoolConfig
	}

	for poolName, poolBody := range poolConfig.Pools {
		tempPool := poolBody
		tempPool.CPUset, err = cpuset.Parse(poolBody.CPUStr)
		if err != nil {
			typesLogger.Error(ErrNotParsePoolConfig.Error(), logger.Error(err))
			return PoolConfig{}, ErrNotParsePoolConfig
		}
		if poolBody.HTPolicy == "" { //Default Set HTPolicy is SingleThreadHTPolicy
			tempPool.HTPolicy = SingleThreadHTPolicy
		}
		poolConfig.Pools[poolName] = tempPool
	}
	return poolConfig, err
}

//DeterminePoolType takes the name of CPU pool as defined in the CPUSets ConfigMap, and returns the type of CPU pool it represents.
//Type of the pool is determined based on the constant prefixes used in the name of the pool.
//A type can be shared, exclusive, or default.
func DeterminePoolType(poolName string) string {
	if strings.HasPrefix(poolName, SharedPoolID) {
		return SharedPoolID
	} else if strings.HasPrefix(poolName, ExclusivePoolID) {
		return ExclusivePoolID
	}
	return DefaultPoolID
}

//DeterminePoolConfig first interrogates the label set of the Node this process runs on.
//It uses this information to select the specific PoolConfig file corresponding to the Node.
//Returns the selected PoolConfig file, the name of the file, or an error if it was impossible to determine which config file is applicable.
func DeterminePoolConfig(k8sclient k8sclient.Interface, fileMatch, nodeName string) (PoolConfig, error) {
	nodeLabels, err := client.GetNodeLabels(k8sclient, nodeName)
	if err != nil {
		typesLogger.Error(ErrCallAPIServerNodeInfo.Error(), logger.Error(err))
		return PoolConfig{}, ErrCallAPIServerNodeInfo
	}
	return parsePoolConfigs(nodeLabels, fileMatch)
}

func parsePoolConfigs(labelMap map[string]string, fileMatch string) (PoolConfig, error) {
	poolConfs, err := readAllPoolConfigs(fileMatch)
	if err != nil {
		typesLogger.Error("Parse configuration file", logger.Error(err))
		return PoolConfig{}, err
	}
	for index, poolConf := range poolConfs {
		if labelMap == nil {
			typesLogger.Info("Using first configuration file as pool config in lieu of missing Node information")
			return poolConf, nil
		}
		for label, labelValue := range labelMap {
			if value, ok := poolConf.NodeSelector[label]; ok {
				if value == labelValue {
					typesLogger.Info("Using configuration file no index for pool config", logger.Any("index", index), logger.Any("value", poolConf))
					return poolConf, nil
				}
			}
		}
	}
	return PoolConfig{}, ErrNotMatchPoolConfig
}

//readAllPoolConfigs reads all the CPU pools configured in the cluster, and returns them to the user in one big array
func readAllPoolConfigs(fileMatch string) ([]PoolConfig, error) {
	files, err := filepath.Glob(filepath.Join(PoolConfigDir, fileMatch))
	if err != nil {
		typesLogger.Error("filepath glob parse error!", logger.Error(err))
		return nil, err
	}
	poolConfs := make([]PoolConfig, 0)
	for _, f := range files {
		poolConf, err := parsePoolConfigFile(f)
		if err != nil {
			return nil, err
		}
		poolConfs = append(poolConfs, poolConf)
	}
	return poolConfs, nil
}
