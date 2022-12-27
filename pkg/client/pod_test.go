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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	testScheme *runtime.Scheme
)

func init() {
	testScheme = runtime.NewScheme()
	_ = v1.AddToScheme(testScheme)
	_ = appsv1.AddToScheme(testScheme)
}

func TestIsCompletePod(t *testing.T) {
	namespace := "default"
	pods := []*v1.Pod{{ObjectMeta: metav1.ObjectMeta{Name: "pod1",
		Namespace: namespace},
		Spec: v1.PodSpec{},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{Name: "pod2",
			Namespace: namespace},
		Spec: v1.PodSpec{},
		Status: v1.PodStatus{
			Phase: v1.PodSucceeded,
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{Name: "pod3",
			Namespace: namespace},
		Spec: v1.PodSpec{},
		Status: v1.PodStatus{
			Phase: v1.PodFailed,
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{Name: "pod4",
			Namespace:         namespace,
			DeletionTimestamp: &metav1.Time{Time: time.Now()}},
		Spec: v1.PodSpec{},
	}}

	type args struct {
		name      string
		namespace string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Pod doesn't exist",
			args: args{
				name:      "notExist",
				namespace: namespace,
			},
			want: false,
		},
		{
			name: "Pod is running",
			args: args{
				name:      "pod1",
				namespace: namespace,
			},
			want: false,
		},
		{
			name: "Pod is succeed",
			args: args{
				name:      "pod2",
				namespace: namespace,
			},
			want: true,
		}, {
			name: "Pod is failed",
			args: args{
				name:      "pod3",
				namespace: namespace,
			},
			want: true,
		}, {
			name: "Pod's deletion timestamp not nil",
			args: args{
				name:      "pod4",
				namespace: namespace,
			},
			want: true,
		},
	}

	testPods := []runtime.Object{}

	for _, pod := range pods {
		testPods = append(testPods, pod.DeepCopy())
	}

	clientset := fake.NewClientBuilder().WithScheme(testScheme).WithRuntimeObjects(testPods...).Build()

	assert := assert.New(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pod v1.Pod
			var podToTest *v1.Pod
			key := types.NamespacedName{
				Namespace: tt.args.namespace,
				Name:      tt.args.name,
			}
			_ = clientset.Get(context.TODO(), key, &pod)

			if len(pod.Name) == 0 {
				podToTest = nil
			} else {
				podToTest = &pod
			}

			assert.Equal(IsCompletePod(podToTest), tt.want)
		})
	}
}

func TestIsFailedPod(t *testing.T) {
	namespace := "default"
	pods := []*v1.Pod{{ObjectMeta: metav1.ObjectMeta{Name: "runningPod",
		Namespace: namespace},
		Spec: v1.PodSpec{},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{Name: "succeedPod",
			Namespace: namespace},
		Spec: v1.PodSpec{},
		Status: v1.PodStatus{
			Phase: v1.PodSucceeded,
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{Name: "failedPod",
			Namespace: namespace},
		Spec: v1.PodSpec{},
		Status: v1.PodStatus{
			Phase: v1.PodFailed,
		},
	}}
	type args struct {
		pod *v1.Pod
	}
	type testcase struct {
		name string
		args args
		want bool
	}

	tests := []testcase{}

	for _, pod := range pods {
		tests = append(tests, testcase{
			name: pod.Name,
			args: args{
				pod: pod,
			},
		})
	}

	tests[0].want = false
	tests[1].want = false
	tests[2].want = true

	assert := assert.New(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(IsFailedPod(tt.args.pod), tt.want)
		})
	}
}

func TestIsSucceededPod(t *testing.T) {
	namespace := "default"
	pods := []*v1.Pod{{ObjectMeta: metav1.ObjectMeta{Name: "runningPod",
		Namespace: namespace},
		Spec: v1.PodSpec{},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{Name: "succeedPod",
			Namespace: namespace},
		Spec: v1.PodSpec{},
		Status: v1.PodStatus{
			Phase: v1.PodSucceeded,
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{Name: "failedPod",
			Namespace: namespace},
		Spec: v1.PodSpec{},
		Status: v1.PodStatus{
			Phase: v1.PodFailed,
		},
	}}
	type args struct {
		pod *v1.Pod
	}
	type testcase struct {
		name string
		args args
		want bool
	}

	tests := []testcase{}

	for _, pod := range pods {
		tests = append(tests, testcase{
			name: pod.Name,
			args: args{
				pod: pod,
			},
		})
	}

	tests[0].want = false
	tests[1].want = true
	tests[2].want = false

	assert := assert.New(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(IsSucceededPod(tt.args.pod), tt.want)
		})
	}
}

func TestGetPodByName(t *testing.T) {
	namespace := "default"
	pods := []*v1.Pod{{ObjectMeta: metav1.ObjectMeta{Name: "pod1",
		Namespace: namespace},
		Spec: v1.PodSpec{},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{Name: "pod2",
			Namespace: namespace},
		Spec: v1.PodSpec{},
		Status: v1.PodStatus{
			Phase: v1.PodSucceeded,
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{Name: "pod3",
			Namespace: namespace},
		Spec: v1.PodSpec{},
		Status: v1.PodStatus{
			Phase: v1.PodFailed,
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{Name: "pod4",
			Namespace:         namespace,
			DeletionTimestamp: &metav1.Time{Time: time.Now()}},
		Spec: v1.PodSpec{},
	}}

	testPods := []runtime.Object{}

	for _, pod := range pods {
		testPods = append(testPods, pod.DeepCopy())
	}

	clientset := k8sfake.NewSimpleClientset(testPods...)

	type args struct {
		name      string
		namespace string
	}
	tests := []struct {
		name string
		args args
		want *v1.Pod
	}{
		{
			name: "Pod doesn't exist",
			args: args{
				name:      "notExist",
				namespace: namespace,
			},
			want: nil,
		},
		{
			name: "Pod is running",
			args: args{
				name:      "pod1",
				namespace: namespace,
			},
			want: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "pod1",
					Namespace: namespace},
				Spec: v1.PodSpec{},
			},
		},
	}

	assert := assert.New(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod, err := GetPodByName(clientset, tt.args.name, tt.args.namespace)

			if tt.want == nil {
				assert.Nil(pod)
				assert.NotNil(err)
			} else {
				assert.Nil(err)
				assert.NotNil(pod)
				assert.Equal(pod.Name, tt.args.name)
				assert.Equal(pod.Namespace, tt.args.namespace)
			}

		})
	}
}

func TestSetPodAnnotation(t *testing.T) {
	namespace := "default"
	pods := []*v1.Pod{{ObjectMeta: metav1.ObjectMeta{Name: "pod1",
		Namespace:   namespace,
		Annotations: map[string]string{"aa": "pod1"},
	},
		Spec: v1.PodSpec{},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{Name: "pod2",
			Namespace:   namespace,
			Annotations: map[string]string{"aa": "pod2"},
		},
		Spec: v1.PodSpec{},
		Status: v1.PodStatus{
			Phase: v1.PodSucceeded,
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{Name: "pod3",
			Namespace:   namespace,
			Annotations: map[string]string{"aa": "pod3"},
		},
		Spec: v1.PodSpec{},
		Status: v1.PodStatus{
			Phase: v1.PodFailed,
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{Name: "pod4",
			Namespace:         namespace,
			DeletionTimestamp: &metav1.Time{Time: time.Now()}},
		Spec: v1.PodSpec{},
	}}

	testPods := []runtime.Object{}

	for _, pod := range pods {
		testPods = append(testPods, pod.DeepCopy())
	}

	clientset := k8sfake.NewSimpleClientset(testPods...)

	type args struct {
		key   string
		value string
		pod   *v1.Pod
	}
	tests := []struct {
		name string
		args args
		want *v1.Pod
	}{
		{
			name: "Pod1 update",
			args: args{
				key:   "aa",
				value: "ddd",
				pod: &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1",
					Namespace:   namespace,
					Annotations: map[string]string{"aa": "pod1"},
				},
					Spec: v1.PodSpec{},
					Status: v1.PodStatus{
						Phase: v1.PodRunning,
					},
				},
			},
			want: &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1",
				Namespace:   namespace,
				Annotations: map[string]string{"aa": "ddd"},
			},
				Spec: v1.PodSpec{},
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
				},
			},
		},
		{
			name: "Pod2 append",
			args: args{
				key:   "ee",
				value: "ff",
				pod: &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod2",
					Namespace:   namespace,
					Annotations: map[string]string{"aa": "pod2"},
				},
					Spec: v1.PodSpec{},
					Status: v1.PodStatus{
						Phase: v1.PodRunning,
					},
				},
			},
			want: &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod2",
				Namespace:   namespace,
				Annotations: map[string]string{"aa": "pod2", "ee": "ff"},
			},
				Spec: v1.PodSpec{},
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
				},
			},
		},
		{
			name: "Pod4 create",
			args: args{
				key:   "ee",
				value: "ff",
				pod: &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "pod4",
						Namespace:         namespace,
						DeletionTimestamp: &metav1.Time{Time: time.Now()}},
					Spec: v1.PodSpec{},
				},
			},
			want: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "pod4",
					Namespace:         namespace,
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
					Annotations:       map[string]string{"ee": "ff"},
				},
				Spec: v1.PodSpec{},
			},
		},
	}

	assert := assert.New(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SetPodAnnotation(clientset, tt.args.pod, tt.args.key, tt.args.value)
			assert.Nil(err)

			pod, err := GetPodByName(clientset, tt.args.pod.ObjectMeta.Name, tt.args.pod.ObjectMeta.Namespace)
			assert.Nil(err)

			assert.Equal(pod.ObjectMeta.Name, tt.want.ObjectMeta.Name)
			assert.Equal(pod.ObjectMeta.Annotations, tt.want.ObjectMeta.Annotations)
		})
	}
}

/*
func TestGetAllPodsByNodeName(t *testing.T) {
	namespace := "default"
	namespace2 := "test1"
	pods := []*v1.Pod{{ObjectMeta: metav1.ObjectMeta{Name: "pod1",
		Namespace: namespace2},
		Spec: v1.PodSpec{
			NodeName: "node1",
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{Name: "pod2",
			Namespace: namespace},
		Spec: v1.PodSpec{
			NodeName: "node1",
		},
		Status: v1.PodStatus{
			Phase: v1.PodSucceeded,
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{Name: "pod3",
			Namespace: namespace2},
		Spec: v1.PodSpec{
			NodeName: "node2",
		},
		Status: v1.PodStatus{
			Phase: v1.PodFailed,
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{Name: "pod4",
			Namespace:         namespace,
			DeletionTimestamp: &metav1.Time{Time: time.Now()}},
		Spec: v1.PodSpec{
			NodeName: "node1",
		},
	}}

	testPods := []runtime.Object{}

	for _, pod := range pods {
		testPods = append(testPods, pod.DeepCopy())
	}

	clientset := k8sfake.NewSimpleClientset(testPods...)

	type args struct {
		nodename string
	}
	tests := []struct {
		name string
		args args
		want *v1.PodList
	}{
		{
			name: "Pod doesn't exist",
			args: args{
				nodename: "notExist",
			},
			want: nil,
		},
		{
			name: "All Pod in node1",
			args: args{
				nodename: "node1",
			},
			want: &v1.PodList{
				Items: []v1.Pod{
					v1.Pod{
						ObjectMeta: metav1.ObjectMeta{Name: "pod1",
							Namespace: namespace2},
						Spec: v1.PodSpec{},
					},
					v1.Pod{
						ObjectMeta: metav1.ObjectMeta{Name: "pod2",
							Namespace: namespace},
						Spec: v1.PodSpec{},
					},
					v1.Pod{
						ObjectMeta: metav1.ObjectMeta{Name: "pod4",
							Namespace: namespace},
						Spec: v1.PodSpec{},
					},
				},
			},
		},
	}

	assert := assert.New(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pods, err := GetAllPodsByNodeName(clientset, tt.args.nodename)

			if tt.want == nil {
				assert.Nil(pods)
				assert.NotNil(err)
			} else {
				assert.Nil(err)
				assert.NotNil(pods)
				assert.Nil(pods.Items)
				assert.Equal(len(pods.Items), 3)
				//assert.Equal(pod.s, tt.args.nodename)
				//assert.Equal(pod.Namespace, tt.args.namespace)
			}

		})
	}
}
*/
