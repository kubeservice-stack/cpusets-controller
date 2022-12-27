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
	"errors"

	"github.com/kubeservice-stack/common/pkg/logger"
)

var typesLogger = logger.GetLogger("pkg/types", "types")

var (
	ErrNoContainerName = errors.New("'container' is mandatory in annotation")
	ErrNoProcesses     = errors.New("'processes' is mandatory in annotation")
	ErrNoProcessName   = errors.New("'process' (name) is mandatory in annotation")
	ErrNoCpus          = errors.New("'cpus' field is mandatory in annotation")

	ErrNotReadPoolConfig  = errors.New("could not read poolconfig file")
	ErrNotParsePoolConfig = errors.New("could not parse poolconfig file")
	ErrNotMatchPoolConfig = errors.New("no matching pool configuration file found for provided nodeSelector label")

	ErrCallAPIServerNodeInfo = errors.New("following error happend when trying to read K8s API server Node object")
)
