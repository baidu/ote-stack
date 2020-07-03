/*
Copyright 2019 Baidu, Inc.

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

package controllermanager

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubernetes "k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"

	"github.com/baidu/ote-stack/pkg/reporter"
)

var (
	podGroup = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	podKind  = schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}
)

func newPodUpdateAction(namespace string, pod *corev1.Pod) kubetesting.UpdateActionImpl {
	return kubetesting.NewUpdateAction(podGroup, namespace, pod)
}

func newPodGetAction(namespace, name string) kubetesting.GetActionImpl {
	return kubetesting.NewGetAction(podGroup, namespace, name)
}

func newPodCreateAction(namespace string, pod *corev1.Pod) kubetesting.CreateActionImpl {
	return kubetesting.NewCreateAction(podGroup, namespace, pod)
}

func newPodDeleteAction(namespace string, name string) kubetesting.DeleteActionImpl {
	return kubetesting.NewDeleteAction(podGroup, namespace, name)
}

func newPodListAction(ops metav1.ListOptions) kubetesting.ListActionImpl {
	return kubetesting.NewListAction(podGroup, podKind, "", ops)
}

func TestHandlePodReport(t *testing.T) {
	u := NewUpstreamProcessor(&K8sContext{})
	u.ctx.K8sClient = &kubernetes.Clientset{}

	podUpdatesMap := &reporter.PodResourceStatus{
		UpdateMap: make(map[string]*corev1.Pod),
		DelMap:    make(map[string]*corev1.Pod),
		FullList:  make([]string, 1),
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-name1",
			Namespace:       "test-namespace1",
			Labels:          map[string]string{reporter.ClusterLabel: "cluster1"},
			ResourceVersion: "10",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
	podUpdatesMap.UpdateMap["test-namespace1/test-name1"] = pod
	podUpdatesMap.DelMap["test-namespace1/test-name2"] = pod
	podUpdatesMap.FullList = []string{"pod1"}

	podUpdatesMapJSON, err := json.Marshal(*podUpdatesMap)
	assert.Nil(t, err)

	err = u.handlePodReport("cluster1", podUpdatesMapJSON)
	assert.Nil(t, err)

	err = u.handlePodReport("cluster1", []byte{1, 2, 3})
	assert.Error(t, err)
}

func TestRetryUpdate(t *testing.T) {
	u := NewUpstreamProcessor(&K8sContext{})

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:                       "test-name1",
			Namespace:                  "test-namespace1",
			Labels:                     map[string]string{reporter.ClusterLabel: "cluster1", reporter.EdgeVersionLabel: "11"},
			ResourceVersion:            "1",
			DeletionTimestamp:          nil,
			DeletionGracePeriodSeconds: nil,
			UID:                        "22",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	getPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:                       "test-name1",
			ResourceVersion:            "4",
			Namespace:                  "test-namespace1",
			Labels:                     map[string]string{reporter.ClusterLabel: "cluster1", reporter.EdgeVersionLabel: "1"},
			DeletionTimestamp:          nil,
			DeletionGracePeriodSeconds: nil,
			UID:                        "22",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	mockClient, tracker := newSimpleClientset(getPod)

	// mock api server ResourceVersion conflict
	mockClient.PrependReactor("update", "pods", func(action kubetesting.Action) (bool, runtime.Object, error) {

		etcdPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:                       "test-name1",
				ResourceVersion:            "9",
				Namespace:                  "test-namespace1",
				Labels:                     map[string]string{reporter.ClusterLabel: "cluster1", reporter.EdgeVersionLabel: "0"},
				DeletionTimestamp:          nil,
				DeletionGracePeriodSeconds: nil,
				UID:                        "22",
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
			},
		}

		if uplPod, ok := action.(kubetesting.UpdateActionImpl); ok {
			if pods, ok := uplPod.Object.(*corev1.Pod); ok {
				// ResourceVersion same length, can be compared with string
				if strings.Compare(etcdPod.ResourceVersion, pods.ResourceVersion) != 0 {
					err := tracker.Update(podGroup, etcdPod, etcdPod.Namespace)
					assert.Nil(t, err)
					return true, nil, kubeerrors.NewConflict(schema.GroupResource{}, "", nil)
				}
			}
		}
		return true, etcdPod, nil
	})

	u.ctx.K8sClient = mockClient
	err := u.UpdatePod(pod)
	assert.Nil(t, err)
}

func TestCreatePod(t *testing.T) {
	u := NewUpstreamProcessor(&K8sContext{})

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-name1",
			Namespace:       "test-namespace1",
			Labels:          map[string]string{reporter.ClusterLabel: "cluster1", reporter.EdgeVersionLabel: "11"},
			ResourceVersion: "1",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	mockClient := &kubernetes.Clientset{}
	mockClient.AddReactor("create", "pods", func(action kubetesting.Action) (bool, runtime.Object, error) {
		return true, pod, nil
	})

	u.ctx.K8sClient = mockClient
	err := u.CreatePod(pod)
	assert.Nil(t, err)

}

func TestGetCreateOrUpdatePod(t *testing.T) {
	testPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "testPod1",
			ResourceVersion: "10",
			Namespace:       "testNamespace",
			Labels:          map[string]string{reporter.EdgeVersionLabel: "11"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Ports: []corev1.ContainerPort{
						{
							Name:          "http",
							ContainerPort: int32(8000)},
					},
				},
			},
		},
	}
	testPod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "testPod1",
			ResourceVersion: "10",
			Namespace:       "testNamespace",
			Labels:          map[string]string{reporter.EdgeVersionLabel: "12"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Ports: []corev1.ContainerPort{
						{
							Name:          "http",
							ContainerPort: int32(8888)},
					},
				},
			},
		},
	}

	tests := []struct {
		name            string
		pod             *corev1.Pod
		getPodResult    *corev1.Pod
		errorOnGet      error
		errorOnCreation error
		errorOnUpdate   error
		errorOnDelete   error
		errorOnPatch    error
		expectActions   []kubetesting.Action
		expectErr       bool
	}{
		{
			name:            "A error occurs when create a new pod fails.",
			pod:             testPod,
			getPodResult:    nil,
			errorOnGet:      kubeerrors.NewNotFound(schema.GroupResource{}, ""),
			errorOnCreation: errors.New("wanted error"),
			expectActions: []kubetesting.Action{
				newPodGetAction(testPod.Namespace, testPod.Name),
				newPodCreateAction(testPod.Namespace, testPod),
			},
			expectErr: true,
		},
		{
			name:            "A error occurs when create an existent pod.",
			pod:             testPod1,
			getPodResult:    testPod,
			errorOnGet:      nil,
			errorOnCreation: nil,
			errorOnUpdate:   errors.New("wanted error"),
			expectActions: []kubetesting.Action{
				newPodGetAction(testPod.Namespace, testPod.Name),
				newPodUpdateAction(testPod.Namespace, testPod),
			},
			expectErr: true,
		},
	}
	u := NewUpstreamProcessor(&K8sContext{})

	for _, test := range tests {
		t.Run(test.name, func(t1 *testing.T) {
			assert := assert.New(t1)

			// Mock.
			mockClient := &kubernetes.Clientset{}
			mockClient.AddReactor("get", "pods", func(action kubetesting.Action) (bool, runtime.Object, error) {
				return true, test.getPodResult, test.errorOnGet
			})
			mockClient.AddReactor("create", "pods", func(action kubetesting.Action) (bool, runtime.Object, error) {
				return true, nil, test.errorOnCreation
			})
			mockClient.AddReactor("update", "pods", func(action kubetesting.Action) (bool, runtime.Object, error) {
				return true, nil, test.errorOnUpdate
			})

			u.ctx.K8sClient = mockClient
			err := u.CreateOrUpdatePod(test.pod)

			if test.expectErr {
				assert.Error(err)
			} else {
				assert.NoError(err)
				// Check calls to kubernetes.
				assert.Equal(test.expectActions, mockClient.Actions())
			}
		})
	}
}

func TestDeletePod(t *testing.T) {
	testPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "testPod1",
			ResourceVersion: "10",
			Namespace:       "testNamespace",
			Labels:          map[string]string{reporter.EdgeVersionLabel: "11"},
		},
	}

	tests := []struct {
		name          string
		pod           *corev1.Pod
		getPodResult  *corev1.Pod
		errorOnGet    error
		errorOnDelete error
		expectActions []kubetesting.Action
		expectErr     bool
	}{
		{
			name:          "Success to delete an existent pod.",
			pod:           testPod,
			getPodResult:  nil,
			errorOnDelete: nil,
			expectActions: []kubetesting.Action{
				newPodDeleteAction(testPod.Namespace, testPod.Name),
			},
			expectErr: false,
		},
		{
			name:          "A error occurs when delete a pod fails.",
			pod:           testPod,
			getPodResult:  nil,
			errorOnDelete: errors.New("wanted error"),
			expectActions: []kubetesting.Action{
				newPodDeleteAction(testPod.Namespace, testPod.Name),
			},
			expectErr: true,
		},
	}
	u := NewUpstreamProcessor(&K8sContext{})

	for _, test := range tests {
		t.Run(test.name, func(t1 *testing.T) {
			assert := assert.New(t1)

			// Mock
			mockClient := &kubernetes.Clientset{}
			mockClient.AddReactor("delete", "pods", func(action kubetesting.Action) (bool, runtime.Object, error) {
				return true, nil, test.errorOnDelete
			})

			u.ctx.K8sClient = mockClient
			err := u.DeletePod(test.pod)

			if test.expectErr {
				assert.Error(err)
			} else {
				assert.NoError(err)
				// Check calls to kubernetes
				assert.Equal(test.expectActions, mockClient.Actions())
			}
		})
	}
}

func TestHandlePodFullList(t *testing.T) {
	fullList := []string{"pod1"}

	ops := metav1.ListOptions{
		LabelSelector: "ote-cluster=c1",
	}

	cmPodList := &corev1.PodList{}
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "pod1-c1",
			Labels: map[string]string{reporter.ClusterLabel: "c1"},
		},
	}
	cmPodList.Items = append(cmPodList.Items, pod)

	cmPodList2 := &corev1.PodList{}
	pod = corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "pod1-c2",
			Labels: map[string]string{reporter.ClusterLabel: "c1"},
		},
	}
	cmPodList2.Items = append(cmPodList.Items, pod)

	tests := []struct {
		name          string
		clusterName   string
		edgePodList   []string
		podList       *corev1.PodList
		expectActions []kubetesting.Action
		expectErr     bool
	}{
		{
			name:        "success to handle full list's pod",
			clusterName: "c1",
			edgePodList: fullList,
			podList:     cmPodList,
			expectActions: []kubetesting.Action{
				newPodListAction(ops),
			},
			expectErr: false,
		},
		{
			name:        "A error occurs when handles a full list pod",
			clusterName: "c1",
			edgePodList: fullList,
			podList:     cmPodList2,
			expectActions: []kubetesting.Action{
				newPodListAction(ops),
				newPodDeleteAction("", "pod1"),
			},
			expectErr: true,
		},
	}

	for _, test := range tests {
		mockClient := &kubernetes.Clientset{}
		mockClient.AddReactor("list", "pods", func(action kubetesting.Action) (bool, runtime.Object, error) {
			return true, test.podList, nil
		})

		u := NewUpstreamProcessor(&K8sContext{})
		u.ctx.K8sClient = mockClient

		u.handlePodFullList(test.clusterName, test.edgePodList)

		if test.expectErr {
			assert.NotEqual(t, test.expectActions, mockClient.Actions())
		} else {
			assert.Equal(t, test.expectActions, mockClient.Actions())
		}
	}
}
