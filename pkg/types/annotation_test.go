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
	"testing"

	"github.com/stretchr/testify/suite"
)

type AnnotationTestSuit struct {
	suite.Suite
	cpuAnnotation CPUAnnotation
}

func (s *AnnotationTestSuit) SetupSuite() {
	//Nothing
}

func (s *AnnotationTestSuit) TearDownSuite() {
	//Nothing
}

func (s *AnnotationTestSuit) SetupTest() {
	s.cpuAnnotation = NewCPUAnnotation()
	s.cpuAnnotation["Container1"] = Container{Name: "Container1", Processes: []Process{
		{ProcName: "proc1", Args: []string{"-c", "1"}, CPUs: 120, PoolName: "shared-pool1"},
		{ProcName: "proc2", Args: []string{"-c", "1"}, CPUs: 1, PoolName: "exclusive-pool2"},
		{ProcName: "proc3", Args: []string{"-c", "1"}, CPUs: 130, PoolName: "shared-pool1"}}}
	s.cpuAnnotation["Container2"] = Container{Name: "Container2", Processes: []Process{
		{ProcName: "proc4", Args: []string{"-c", "1"}, CPUs: 120, PoolName: "shared-pool1"},
		{ProcName: "proc5", Args: []string{"-c", "1"}, CPUs: 1, PoolName: "exclusive-pool2"},
		{ProcName: "proc6", Args: []string{"-c", "1"}, CPUs: 130, PoolName: "shared-pool1"},
		{ProcName: "proc7", Args: []string{"-c", "1"}, CPUs: 300, PoolName: "shared-pool3"}}}
}

func (s *AnnotationTestSuit) TearDownTest() {
	s.cpuAnnotation = NewCPUAnnotation()
}

func (s *AnnotationTestSuit) TestGetContainerPools() {
	pools := s.cpuAnnotation.ContainerPools("Container1")

	s.ElementsMatch([]string{"shared-pool1", "exclusive-pool2"}, pools)
}

func (s *AnnotationTestSuit) TestGetContainerCpuRequest() {
	value := s.cpuAnnotation.ContainerTotalCPURequest("shared-pool1", "Container2")
	s.Equal(value, 250)
}

func (s *AnnotationTestSuit) TestGetContainerNames() {
	s.ElementsMatch([]string{"Container1", "Container2"}, s.cpuAnnotation.ContainerNames())
}

func (s *AnnotationTestSuit) TestContainerSharedCPUTime() {
	v := s.cpuAnnotation.ContainerSharedCPUTime("Container2")
	s.Equal(v, 550)
}

func (s *AnnotationTestSuit) TestContainerDecodeAnnotation() {
	var podannotation = []byte(`[{"container": "cputestcontainer","processes":  [{"process": "/bin/sh","args": ["-c","/thread_busyloop"], "cpus": 1,"pool": "shared-pool1"},{"process": "/bin/sh","args": ["-c","/thread_busyloop2"], "cpus": 2,"pool": "exclusive-pool2"} ] } ]`)
	ca := CPUAnnotation{}
	ca.Decode([]byte(podannotation))
	pools := ca.ContainerPools("cputestcontainer")
	s.ElementsMatch([]string{"shared-pool1", "exclusive-pool2"}, pools)

}

func (s *AnnotationTestSuit) TestContainerDecodeAnnotationUnmarshalFail() {
	var podannotationFail = []byte(`["container": "cputestcontainer","processes":  [{"process": "/bin/sh","args": ["-c","/thread_busyloop"], "cpus": 1,"pool": "cpupool1"},{"process": "/bin/sh","args": ["-c","/thread_busyloop2"], "cpus": 2,"pool": "cpupool2"} ] } ]`)
	ca := CPUAnnotation{}
	err := ca.Decode([]byte(podannotationFail))
	s.NotNil(err)
}

func (s *AnnotationTestSuit) TestContainerDecodeAnnotationNoContainerName() {
	var podannotation = []byte(`[{"processes":  [{"process": "/bin/sh","args": ["-c","/thread_busyloop"], "cpus": 1,"pool": "pool1"},{"process": "/bin/sh","args": ["-c","/thread_busyloop2"], "cpus": 2,"pool": "pool2"} ] } ]`)
	ca := CPUAnnotation{}
	err := ca.Decode([]byte(podannotation))
	s.NotNil(err)
	s.Equal(err, ErrNoContainerName)
}

func (s *AnnotationTestSuit) TestContainerDecodeAnnotationNoProcessName() {
	var podannotation = []byte(`[{"container": "cputestcontainer","processes":  [{"args": ["-c","/thread_busyloop"], "cpus": 1,"pool": "pool1"},{"process": "/bin/sh","args": ["-c","/thread_busyloop2"], "cpus": 2,"pool": "pool2"} ] } ]`)
	ca := CPUAnnotation{}
	err := ca.Decode([]byte(podannotation))
	s.NotNil(err)
	s.Equal(err, ErrNoProcessName)
}

func (s *AnnotationTestSuit) TestContainerDecodeAnnotationNoProcesses() {
	var podannotation = []byte(`[{"container": "cputestcontainer" } ]`)
	ca := CPUAnnotation{}
	err := ca.Decode([]byte(podannotation))
	s.NotNil(err)
	s.Equal(err, ErrNoProcesses)
}

func (s *AnnotationTestSuit) TestContainerDecodeAnnotationNoCpus() {
	var podannotation = []byte(`[{"container": "cputestcontainer","processes":  [{"process": "/bin/sh","args": ["-c","/thread_busyloop"], "pool": "pool1"},{"process": "/bin/sh","args": ["-c","/thread_busyloop2"], "cpus": 2,"pool": "pool2"} ] } ]`)
	ca := CPUAnnotation{}
	err := ca.Decode([]byte(podannotation))
	s.NotNil(err)
	s.Equal(err, ErrNoCpus)
}

// go test 入口
func TestAnnotationTestSuite(t *testing.T) {
	suite.Run(t, new(AnnotationTestSuit))
}
