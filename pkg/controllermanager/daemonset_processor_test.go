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
	appsv1 "k8s.io/api/apps/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubernetes "k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"

	"github.com/baidu/ote-stack/pkg/reporter"
)

var (
	daemonsetGroup = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}
	daemonsetKind  = schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DaemonSet"}
)

func newDaemonSetGetAction(name string) kubetesting.GetActionImpl {
	return kubetesting.NewGetAction(daemonsetGroup, "", name)
}

func newDaemonSetCreateAction(daemonset *appsv1.DaemonSet) kubetesting.CreateActionImpl {
	return kubetesting.NewCreateAction(daemonsetGroup, "", daemonset)
}

func newDaemonSetUpdateAction(daemonset *appsv1.DaemonSet) kubetesting.UpdateActionImpl {
	return kubetesting.NewUpdateAction(daemonsetGroup, "", daemonset)
}

func newDaemonSetDeleteAction(name string) kubetesting.DeleteActionImpl {
	return kubetesting.NewDeleteAction(daemonsetGroup, "", name)
}

func newDaemonSetListAction(ops metav1.ListOptions) kubetesting.ListActionImpl {
	return kubetesting.NewListAction(daemonsetGroup, daemonsetKind, "", ops)
}

func NewDaemonset(name string,
	clusterLabel string, edgeVersion string, resourceVersion string) *appsv1.DaemonSet {
	daemonset := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Labels:          map[string]string{reporter.ClusterLabel: clusterLabel, reporter.EdgeVersionLabel: edgeVersion},
			ResourceVersion: resourceVersion,
		},
	}

	return daemonset
}

func TestHandleDaemonsetReport(t *testing.T) {
	u := NewUpstreamProcessor(&K8sContext{})
	u.ctx.K8sClient = &kubernetes.Clientset{}

	daemonsetUpdateMap := &reporter.DaemonsetResourceStatus{
		UpdateMap: make(map[string]*appsv1.DaemonSet),
		DelMap:    make(map[string]*appsv1.DaemonSet),
		FullList:  make([]string, 1),
	}

	daemonset := NewDaemonset("test-name1", "cluster1", "", "1")

	daemonsetUpdateMap.UpdateMap["test-namespace1/test-name1"] = daemonset
	daemonsetUpdateMap.DelMap["test-namespace1/test-name2"] = daemonset
	daemonsetUpdateMap.FullList = []string{"ds1"}

	daemonsetJson, err := json.Marshal(daemonsetUpdateMap)
	assert.Nil(t, err)

	err = u.handleDaemonsetReport("cluster1", daemonsetJson)
	assert.Nil(t, err)

	err = u.handleDaemonsetReport("cluster1", []byte{1})
	assert.NotNil(t, err)
}

func TestRetryDaemonsetUpdate(t *testing.T) {
	u := NewUpstreamProcessor(&K8sContext{})

	daemonset := NewDaemonset("test-name1", "cluster1", "11", "1")
	getDaemonset := NewDaemonset("test-name1", "cluster1", "1", "4")

	mockClient, tracker := newSimpleClientset(getDaemonset)

	// mock api server ResourceVersion conflict
	mockClient.PrependReactor("update", "daemonsets", func(action kubetesting.Action) (bool, runtime.Object, error) {
		etcdDaemonset := NewDaemonset("test-name1", "cluster1", "0", "9")

		if updateDaemonset, ok := action.(kubetesting.UpdateActionImpl); ok {
			if daemonsets, ok := updateDaemonset.Object.(*appsv1.DaemonSet); ok {
				// ResourceVersion same length, can be compared with string
				if strings.Compare(etcdDaemonset.ResourceVersion, daemonsets.ResourceVersion) != 0 {
					err := tracker.Update(daemonsetGroup, etcdDaemonset, "")
					assert.Nil(t, err)
					return true, nil, kubeerrors.NewConflict(schema.GroupResource{}, "", nil)
				}
			}
		}
		return true, etcdDaemonset, nil
	})

	u.ctx.K8sClient = mockClient
	err := u.UpdateDaemonset(daemonset)
	assert.Nil(t, err)
}

func TestGetCreateOrUpdateDaemonset(t *testing.T) {
	daemonset1 := NewDaemonset("test1", "", "11", "10")

	tests := []struct {
		name               string
		daemonset          *appsv1.DaemonSet
		getDaemonSetResult *appsv1.DaemonSet
		errorOnGet         error
		errorOnCreation    error
		errorOnUpdate      error
		expectActions      []kubetesting.Action
		expectErr          bool
	}{
		{
			name:       "A error occurs when create a daemonset.",
			daemonset:  daemonset1,
			errorOnGet: kubeerrors.NewNotFound(schema.GroupResource{}, ""),
			expectActions: []kubetesting.Action{
				newDaemonSetGetAction(daemonset1.Name),
				newDaemonSetCreateAction(daemonset1),
				newDaemonSetGetAction(daemonset1.Name),
			},
			expectErr: true,
		},
	}

	u := NewUpstreamProcessor(&K8sContext{})

	for _, test := range tests {
		t.Run(test.name, func(t1 *testing.T) {
			assert := assert.New(t1)

			mockClient := &kubernetes.Clientset{}
			mockClient.AddReactor("get", "daemonsets", func(action kubetesting.Action) (bool, runtime.Object, error) {
				return true, test.getDaemonSetResult, test.errorOnGet
			})
			mockClient.AddReactor("create", "daemonsets", func(action kubetesting.Action) (bool, runtime.Object, error) {
				return true, nil, test.errorOnCreation
			})

			u.ctx.K8sClient = mockClient
			err := u.CreateOrUpdateDaemonset(test.daemonset)

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

func TestDeleteDaemonset(t *testing.T) {
	testDaemonset := NewDaemonset("test1", "", "11", "10")

	tests := []struct {
		name               string
		daemonset          *appsv1.DaemonSet
		getDaemonSetResult *appsv1.DaemonSet
		errorOnGet         error
		errorOnDelete      error
		expectActions      []kubetesting.Action
		expectErr          bool
	}{
		{
			name:      "Success to delete an existent daemonset.",
			daemonset: testDaemonset,
			expectActions: []kubetesting.Action{
				newDaemonSetDeleteAction(testDaemonset.Name),
			},
			expectErr: false,
		},
		{
			name:          "A error occurs when delete a daemonset fails.",
			daemonset:     testDaemonset,
			errorOnDelete: errors.New("wanted error"),
			expectActions: []kubetesting.Action{
				newDaemonSetDeleteAction(testDaemonset.Name),
			},
			expectErr: true,
		},
	}

	u := NewUpstreamProcessor(&K8sContext{})

	for _, test := range tests {
		t.Run(test.name, func(t1 *testing.T) {
			assert := assert.New(t1)

			mockClient := &kubernetes.Clientset{}
			mockClient.AddReactor("delete", "daemonsets", func(action kubetesting.Action) (bool, runtime.Object, error) {
				return true, nil, test.errorOnDelete
			})

			u.ctx.K8sClient = mockClient
			err := u.DeleteDaemonset(test.daemonset)

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

func TestHandleDaemonsetFullList(t *testing.T) {
	fullList := []string{"ds1"}

	ops := metav1.ListOptions{
		LabelSelector: "ote-cluster=c1",
	}

	cmDaemonsetList := &appsv1.DaemonSetList{}
	daemonset := NewDaemonset("ds1-c1", "c1", "", "")
	cmDaemonsetList.Items = append(cmDaemonsetList.Items, *daemonset)

	cmDaemonsetList2 := &appsv1.DaemonSetList{}
	daemonset = NewDaemonset("ds2-c1", "c1", "", "")
	cmDaemonsetList2.Items = append(cmDaemonsetList2.Items, *daemonset)

	tests := []struct {
		name              string
		clusterName       string
		edgeDaemonsetList []string
		daemonsetList     *appsv1.DaemonSetList
		expectActions     []kubetesting.Action
		expectErr         bool
	}{
		{
			name:              "success to handle full list's daemonset",
			clusterName:       "c1",
			edgeDaemonsetList: fullList,
			daemonsetList:     cmDaemonsetList,
			expectActions: []kubetesting.Action{
				newDaemonSetListAction(ops),
			},
			expectErr: false,
		},
		{
			name:              "A error occurs when handles a full list daemonset",
			clusterName:       "c1",
			edgeDaemonsetList: fullList,
			daemonsetList:     cmDaemonsetList2,
			expectActions: []kubetesting.Action{
				newDaemonSetListAction(ops),
				newDaemonSetDeleteAction("ds2"),
			},
			expectErr: true,
		},
	}

	for _, test := range tests {
		mockClient := &kubernetes.Clientset{}
		mockClient.AddReactor("list", "daemonsets", func(action kubetesting.Action) (bool, runtime.Object, error) {
			return true, test.daemonsetList, nil
		})

		u := NewUpstreamProcessor(&K8sContext{})
		u.ctx.K8sClient = mockClient

		u.handleDaemonsetFullList(test.clusterName, test.edgeDaemonsetList)

		if test.expectErr {
			assert.NotEqual(t, test.expectActions, mockClient.Actions())
		} else {
			assert.Equal(t, test.expectActions, mockClient.Actions())
		}
	}
}
