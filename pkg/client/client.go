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
	"encoding/json"

	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Global variables
var (
	Clientset  *kubernetes.Clientset
	RestConfig *restclient.Config
	retries    = 3
)

type Meta struct {
	Annotations map[string]json.RawMessage `json:"annotations"`
}

type Update struct {
	Metadata Meta `json:"metadata"`
}

func KubeClient() error {
	var err error
	RestConfig, err = restclient.InClusterConfig()
	if err != nil {
		return err
	}
	Clientset, err = kubernetes.NewForConfig(RestConfig)
	if err != nil {
		return err
	}
	return nil
}

func KubeConfigClientSet(kubeConf string) (*kubernetes.Clientset, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeConf)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}
