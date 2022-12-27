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

package client

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"

	"k8s.io/apimachinery/pkg/types"
	k8sclient "k8s.io/client-go/kubernetes"
)

// IsCompletePod determines if the pod is complete
func IsCompletePod(pod *v1.Pod) bool {
	if pod == nil {
		return false
	}

	if pod.DeletionTimestamp != nil {
		return true
	}

	if pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed {
		return true
	}
	return false
}

// IsSucceededPod determines if the pod is Succeeded
func IsSucceededPod(pod *v1.Pod) bool {
	return pod != nil && pod.Status.Phase == v1.PodSucceeded
}

// IsFailedPod determines if the pod is failed
func IsFailedPod(pod *v1.Pod) bool {
	return pod != nil && pod.Status.Phase == v1.PodFailed
}

// SetPodAnnotation adds or modifies annotation for pod
func SetPodAnnotation(k8sclient k8sclient.Interface, pod *v1.Pod, key string, value string) error {
	merge := Update{}
	merge.Metadata.Annotations = make(map[string]json.RawMessage)
	merge.Metadata.Annotations[key] = json.RawMessage(`"` + value + `"`)

	jsonData, err := json.Marshal(merge)
	if err != nil {
		return err
	}
	_, err = k8sclient.CoreV1().Pods(pod.ObjectMeta.Namespace).Patch(context.TODO(), pod.ObjectMeta.Name, types.MergePatchType, jsonData, metav1.PatchOptions{})
	return err
}

// filterActivePods filter active pods
func filterActivePods(pods *v1.PodList) *v1.PodList {
	activePods := []v1.Pod{}
	for _, pod := range pods.Items {
		if pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed {
			continue
		}

		activePods = append(activePods, pod)
	}

	pods.Items = activePods
	return pods
}

// GetPodByName gets pod with given name and namespace of the pod.
func GetPodByName(k8sclient k8sclient.Interface, name, namespace string) (*v1.Pod, error) {
	return k8sclient.CoreV1().Pods(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

// RefreshPod takes an existing Pod object as an input, and re-reads it from the K8s API
// Returns the refreshed Pod descriptor in case of success, or an error
func RefreshPod(k8sclient k8sclient.Interface, pod v1.Pod) (*v1.Pod, error) {
	return k8sclient.CoreV1().Pods(pod.ObjectMeta.Namespace).Get(context.TODO(), pod.ObjectMeta.Name, metav1.GetOptions{})
}

// GetAllPodsByNodeName gets pod with given Node name.
func GetAllPodsByNodeName(k8sclient k8sclient.Interface, nodeName string) (*v1.PodList, error) {
	selector := fields.SelectorFromSet(fields.Set{"spec.nodeName": nodeName})
	pods, err := k8sclient.CoreV1().Pods(v1.NamespaceAll).List(context.TODO(), metav1.ListOptions{
		FieldSelector: selector.String(),
		LabelSelector: labels.Everything().String(),
	})

	for i := 0; i < retries && err != nil; i++ {
		pods, err = k8sclient.CoreV1().Pods(v1.NamespaceAll).List(context.TODO(), metav1.ListOptions{
			FieldSelector: selector.String(),
			LabelSelector: labels.Everything().String(),
		})
		time.Sleep(100 * time.Millisecond)
	}

	if err != nil {
		return nil, errors.New("failed to get Pods in node" + nodeName)
	}

	return pods, nil
}

// GetActivePodsByNodeName gets pod with given Node name.
func GetActivePodsByNodeName(k8sclient k8sclient.Interface, nodeName string) (*v1.PodList, error) {
	pods, err := GetAllPodsByNodeName(k8sclient, nodeName)
	if err != nil {
		return nil, err
	}

	return filterActivePods(pods), nil
}
