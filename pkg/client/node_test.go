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
	"testing"

	"github.com/stretchr/testify/assert"

	v1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestIsReady(t *testing.T) {

	testNodeInputs := []*v1.Node{{
		ObjectMeta: metav1.ObjectMeta{Name: "test1"},
		Status: v1.NodeStatus{
			Conditions: []v1.NodeCondition{
				{
					Type:               v1.NodeReady,
					Status:             v1.ConditionTrue,
					Reason:             "FakeReady",
					LastTransitionTime: metav1.Now(),
					LastHeartbeatTime:  metav1.Now(),
				},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{Name: "test2"},
		Status: v1.NodeStatus{
			Conditions: []v1.NodeCondition{
				{
					Type:               v1.NodeReady,
					Status:             v1.ConditionFalse,
					Reason:             "FakePending",
					LastTransitionTime: metav1.Now(),
					LastHeartbeatTime:  metav1.Now(),
				},
			},
		},
	}}

	type args struct {
		node v1.Node
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Node Ready",
			args: args{
				node: *testNodeInputs[0],
			},
			want: true,
		}, {
			name: "Node not Ready",
			args: args{
				node: *testNodeInputs[1],
			},
			want: false,
		},
	}

	assert := assert.New(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(IsReady(tt.args.node), tt.want)
		})
	}
}

func TestGetNode(t *testing.T) {
	testNodeInputs := []*v1.Node{{
		ObjectMeta: metav1.ObjectMeta{Name: "test1"},
	}, {
		ObjectMeta: metav1.ObjectMeta{Name: "test2"},
	}}

	testNodes := []runtime.Object{}

	for _, ns := range testNodeInputs {
		testNodes = append(testNodes, ns.DeepCopy())
	}

	//client := fake.NewFakeClientWithScheme(testScheme, testNodes...)
	clientset := k8sfake.NewSimpleClientset(testNodes...)

	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
		want *v1.Node
	}{
		{
			name: "Node doesn't exist",
			args: args{
				name: "notExist",
			},
			want: nil,
		},
		{
			name: "Node exists",
			args: args{
				name: "test1",
			},
			want: testNodeInputs[0].DeepCopy(),
		},
	}

	assert := assert.New(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, _ := GetNode(clientset, tt.args.name)

			assert.Equal(want, tt.want)
			if tt.want != nil {
				assert.Equal(want.Name, tt.args.name)
			}

		})
	}
}

func TestGetNodeLabels(t *testing.T) {
	testNodeInputs := []*v1.Node{{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "test1",
			Labels: map[string]string{"aa": "bb"},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{Name: "test2"},
	}}

	testNodes := []runtime.Object{}

	for _, ns := range testNodeInputs {
		testNodes = append(testNodes, ns.DeepCopy())
	}

	//client := fake.NewFakeClientWithScheme(testScheme, testNodes...)
	clientset := k8sfake.NewSimpleClientset(testNodes...)

	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "Node doesn't exist",
			args: args{
				name: "notExist",
			},
			want: nil,
		},
		{
			name: "Node1 exists",
			args: args{
				name: "test1",
			},
			want: map[string]string{"aa": "bb"},
		},
		{
			name: "Node2 exists",
			args: args{
				name: "test2",
			},
			want: nil,
		},
	}

	assert := assert.New(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, _ := GetNodeLabels(clientset, tt.args.name)
			assert.Equal(want, tt.want)
		})
	}
}
