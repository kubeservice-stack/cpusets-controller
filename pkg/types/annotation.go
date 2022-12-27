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
	"encoding/json"
	"strings"

	"github.com/kubeservice-stack/common/pkg/logger"
)

// Process defines Process Information in pod annotation
// The information is used for setting CPU-Affinity
type Process struct {
	ProcName string   `json:"process"`
	Args     []string `json:"args"`
	CPUs     int      `json:"cpus"`
	PoolName string   `json:"pool"`
}

// Container idenfifies container and defines the processes to be started
type Container struct {
	Name      string    `json:"container"`
	Processes []Process `json:"processes"`
}

type CPUAnnotation map[string]Container

// NewCPUAnnotation returns a new CPUAnnotation
func NewCPUAnnotation() CPUAnnotation {
	return make(CPUAnnotation)
}

// Containers returns container name string in annotation
func (cpuAnnotation CPUAnnotation) ContainerNames() []string {
	var names []string

	for _, cont := range cpuAnnotation {
		names = append(names, cont.Name)
	}
	return names
}

// IsContainerExists tells if container exist in annotation
func (cpuAnnotation CPUAnnotation) IsContainerExists(name string) bool {
	_, exists := cpuAnnotation[name]
	return exists
}

// ContainerSharedCPUTime returns sum of cpu time requested from shared pool by a container
func (cpuAnnotation CPUAnnotation) ContainerSharedCPUTime(cName string) int {
	var cpuTime int

	for _, container := range cpuAnnotation {
		if container.Name == cName {
			for _, process := range container.Processes {
				if strings.HasPrefix(process.PoolName, "shared") {
					cpuTime += process.CPUs
				}
			}
		}
	}
	return cpuTime

}

// ContainerExclusiveCPU returns sum of cpu time requested from exclusive pool by a container
func (cpuAnnotation CPUAnnotation) ContainerExclusiveCPU(cName string) int {
	var cpuTime int

	for _, container := range cpuAnnotation {
		if container.Name == cName {
			for _, process := range container.Processes {
				if strings.HasPrefix(process.PoolName, "exclusive") {
					cpuTime += process.CPUs
				}
			}
		}
	}
	return cpuTime
}

// ContainerPools returns all pools configured for container
func (cpuAnnotation CPUAnnotation) ContainerPools(cName string) (pools []string) {
	var poolMap = make(map[string]bool)
	for _, container := range cpuAnnotation {
		if container.Name == cName {
			for _, process := range container.Processes {
				if _, ok := poolMap[process.PoolName]; !ok {
					pools = append(pools, process.PoolName)
					poolMap[process.PoolName] = true
				}
			}
		}
	}
	return pools
}

// ContainerTotalCPURequest returns CPU requests of container from pool
func (cpuAnnotation CPUAnnotation) ContainerTotalCPURequest(pool string, cName string) int {
	var cpuRequest int
	for _, container := range cpuAnnotation {
		if container.Name == cName {
			for _, process := range container.Processes {
				if process.PoolName == pool {
					cpuRequest += process.CPUs
				}
			}
		}
	}
	return cpuRequest
}

// Decode unmarshals json annotation to CPUAnnotation
func (cpuAnnotation CPUAnnotation) Decode(annotation []byte) error {
	// The annotation in pod spec could be a map but for now
	// it is kept as an array for backwards compatibility
	containers := make([]Container, 0)
	err := json.Unmarshal(annotation, &containers)
	for _, container := range containers {
		cpuAnnotation[container.Name] = container
	}
	if err != nil {
		typesLogger.Error("CPUAnnotation Decode Error!", logger.Error(err))
		return err
	}
	for _, c := range cpuAnnotation {
		if len(c.Name) == 0 {
			return ErrNoContainerName
		}
		if len(c.Processes) == 0 {
			return ErrNoProcesses

		}
		for _, p := range c.Processes {
			if len(p.ProcName) == 0 {
				return ErrNoProcessName

			}
			if p.CPUs == 0 {
				return ErrNoCpus

			}
		}
	}
	return nil
}
