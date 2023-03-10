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

package checkpoint

import (
	cp "k8s.io/kubernetes/pkg/kubelet/cm/devicemanager/checkpoint"
)

// PodDevicesEntry is representing Pod specific deviceID allocations from kubelet checkpoint file structure - valid until K8s 1.20
// TODO: REMOVE THIS TPYE AFTER 1.20 SUPPORT IS DROPPED
type PodDevicesEntry struct {
	PodUID        string
	ContainerName string
	ResourceName  string
	DeviceIDs     []string
}

// NewPodDevicesEntry is representing Pod specific deviceID allocations from kubelet checkpoint file structure - valid from K8s 1.21 onward
// Reference: https://github.com/kubernetes/kubernetes/commit/a8b8995ef241e93e9486d475126450f33f24ef4e
type NewPodDevicesEntry struct {
	PodUID        string
	ContainerName string
	ResourceName  string
	DeviceIDs     cp.DevicesPerNUMA //map[int64][]string
}

// File is representing the kubelet checkpoint file structure with only relevant fields - valid until K8s 1.20
// TODO: REMOVE THIS TPYE AFTER 1.20 SUPPORT IS DROPPED
type File struct {
	Data struct {
		PodDeviceEntries []PodDevicesEntry
	}
}

// NewFile is representing the kubelet checkpoint file structure with only relevant fields - valid from K8s 1.21 onward
// Reference: https://github.com/kubernetes/kubernetes/commit/a8b8995ef241e93e9486d475126450f33f24ef4e
type NewFile struct {
	Data struct {
		PodDeviceEntries []NewPodDevicesEntry
	}
}

// TranslateNewCheckpointToOld downgrades from an 1.21+ checkpoint file representation to the old format
// It simply merges all the NUMA specififc DeviceID string slices into one big slice
// Enables code re-use without needing to modify the business logic of an Operator needing to simultaneously support pre, and post 1.21 K8s versions
func TranslateNewCheckpointToOld(newFile NewFile) File {
	var oldFile File
	oldFile.Data.PodDeviceEntries = make([]PodDevicesEntry, len(newFile.Data.PodDeviceEntries))
	for entryID, podDeviceEntry := range newFile.Data.PodDeviceEntries {
		oldFile.Data.PodDeviceEntries[entryID].PodUID = podDeviceEntry.PodUID
		oldFile.Data.PodDeviceEntries[entryID].ContainerName = podDeviceEntry.ContainerName
		oldFile.Data.PodDeviceEntries[entryID].ResourceName = podDeviceEntry.ResourceName
		for _, devicesPerNUMA := range podDeviceEntry.DeviceIDs {
			oldFile.Data.PodDeviceEntries[entryID].DeviceIDs = append(oldFile.Data.PodDeviceEntries[entryID].DeviceIDs, devicesPerNUMA...)
		}
	}
	return oldFile
}
