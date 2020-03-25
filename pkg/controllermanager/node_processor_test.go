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
	nodeGroup = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes"}
	nodeKind  = schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Node"}
)

func newNodeUpdateAction(node *corev1.Node) kubetesting.UpdateActionImpl {
	return kubetesting.NewUpdateAction(nodeGroup, "", node)
}

func newNodeGetAction(name string) kubetesting.GetActionImpl {
	return kubetesting.NewGetAction(nodeGroup, "", name)
}

func newNodeCreateAction(node *corev1.Node) kubetesting.CreateActionImpl {
	return kubetesting.NewCreateAction(nodeGroup, "", node)
}

func newNodeDeleteAction(name string) kubetesting.DeleteActionImpl {
	return kubetesting.NewDeleteAction(nodeGroup, "", name)
}

func newNodeListAction(ops metav1.ListOptions) kubetesting.ListActionImpl {
	return kubetesting.NewListAction(nodeGroup, nodeKind, "", ops)
}

func TestHandleNodeReport(t *testing.T) {
	u := NewUpstreamProcessor(&K8sContext{})
	u.ctx.K8sClient = &kubernetes.Clientset{}

	nodeUpdatesMap := &reporter.NodeResourceStatus{
		UpdateMap: make(map[string]*corev1.Node),
		DelMap:    make(map[string]*corev1.Node),
		FullList:  make([]string, 1),
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-name1",
			Labels:          map[string]string{reporter.ClusterLabel: "cluster1"},
			ResourceVersion: "10",
		},
		Status: corev1.NodeStatus{
			Phase: corev1.NodeRunning,
		},
	}
	nodeUpdatesMap.UpdateMap["test-namespace1/test-name1"] = node
	nodeUpdatesMap.DelMap["test-namespace1/test-name2"] = node
	nodeUpdatesMap.FullList = []string{"node1"}

	nodeUpdatesMapJSON, err := json.Marshal(nodeUpdatesMap)
	assert.Nil(t, err)

	err = u.handleNodeReport("cluster1", nodeUpdatesMapJSON)
	assert.Nil(t, err)

	err = u.handleNodeReport("cluster1", []byte{1, 2, 3})
	assert.Error(t, err)
}

func TestRetryNodeUpdate(t *testing.T) {
	u := NewUpstreamProcessor(&K8sContext{})

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-name1",
			Labels:          map[string]string{reporter.ClusterLabel: "cluster1", reporter.EdgeVersionLabel: "11"},
			ResourceVersion: "1",
		},
		Status: corev1.NodeStatus{
			Phase: corev1.NodeRunning,
		},
	}

	getNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-name1",
			ResourceVersion: "4",
			Labels:          map[string]string{reporter.ClusterLabel: "cluster1", reporter.EdgeVersionLabel: "1"},
		},
	}

	mockClient, tracker := newSimpleClientset(getNode)

	// mock api server ResourceVersion conflict
	mockClient.PrependReactor("update", "nodes", func(action kubetesting.Action) (bool, runtime.Object, error) {

		etcdNode := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "test-name1",
				ResourceVersion: "9",
				Labels:          map[string]string{reporter.ClusterLabel: "cluster1", reporter.EdgeVersionLabel: "0"},
			},
		}
		if uNode, ok := action.(kubetesting.UpdateActionImpl); ok {
			if nodes, ok := uNode.Object.(*corev1.Node); ok {
				// ResourceVersion same length, can be compared with string
				if strings.Compare(etcdNode.ResourceVersion, nodes.ResourceVersion) != 0 {
					err := tracker.Update(nodeGroup, etcdNode, "")
					assert.Nil(t, err)
					return true, nil, kubeerrors.NewConflict(schema.GroupResource{}, "", nil)
				}
			}
		}
		return true, etcdNode, nil
	})

	u.ctx.K8sClient = mockClient
	err := u.UpdateNode(node)
	assert.Nil(t, err)
}

func TestGetCreateOrUpdateNode(t *testing.T) {
	testNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "testNode1",
			ResourceVersion: "10",
			Labels:          map[string]string{reporter.EdgeVersionLabel: "11"},
		},
	}
	testNode1 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "testNode1",
			ResourceVersion: "10",
			Labels:          map[string]string{reporter.EdgeVersionLabel: "12"},
		},
	}

	tests := []struct {
		name            string
		node            *corev1.Node
		getNodeResult   *corev1.Node
		errorOnGet      error
		errorOnCreation error
		errorOnUpdate   error
		errorOnDelete   error
		expectActions   []kubetesting.Action
		expectErr       bool
	}{
		{
			name:            "Success to create a new node.",
			node:            testNode,
			getNodeResult:   nil,
			errorOnGet:      kubeerrors.NewNotFound(schema.GroupResource{}, ""),
			errorOnCreation: nil,
			expectActions: []kubetesting.Action{
				newNodeGetAction(testNode.Name),
				newNodeCreateAction(testNode),
			},
			expectErr: false,
		},
		{
			name:            "A error occurs when create a new node fails.",
			node:            testNode,
			getNodeResult:   nil,
			errorOnGet:      kubeerrors.NewNotFound(schema.GroupResource{}, ""),
			errorOnCreation: errors.New("wanted error"),
			expectActions: []kubetesting.Action{
				newNodeGetAction(testNode.Name),
				newNodeCreateAction(testNode),
			},
			expectErr: true,
		},
		{
			name:            "A error occurs when create an existent node.",
			node:            testNode1,
			getNodeResult:   testNode,
			errorOnGet:      nil,
			errorOnCreation: nil,
			errorOnUpdate:   errors.New("wanted error"),
			expectActions: []kubetesting.Action{
				newNodeGetAction(testNode.Name),
				newNodeUpdateAction(testNode),
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
			mockClient.AddReactor("get", "nodes", func(action kubetesting.Action) (bool, runtime.Object, error) {
				return true, test.getNodeResult, test.errorOnGet
			})
			mockClient.AddReactor("create", "nodes", func(action kubetesting.Action) (bool, runtime.Object, error) {
				return true, nil, test.errorOnCreation
			})
			mockClient.AddReactor("update", "nodes", func(action kubetesting.Action) (bool, runtime.Object, error) {
				return true, nil, test.errorOnUpdate
			})

			u.ctx.K8sClient = mockClient
			err := u.CreateOrUpdateNode(test.node)

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

func TestDeleteNode(t *testing.T) {
	testNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "testNode1",
			ResourceVersion: "10",
			Labels:          map[string]string{reporter.EdgeVersionLabel: "11"},
		},
	}

	tests := []struct {
		name          string
		node          *corev1.Node
		getNodeResult *corev1.Node
		errorOnGet    error
		errorOnDelete error
		expectActions []kubetesting.Action
		expectErr     bool
	}{
		{
			name:          "Success to delete an existent node.",
			node:          testNode,
			getNodeResult: nil,
			errorOnDelete: nil,
			expectActions: []kubetesting.Action{
				newNodeDeleteAction(testNode.Name),
			},
			expectErr: false,
		},
		{
			name:          "A error occurs when delete a node fails.",
			node:          testNode,
			getNodeResult: nil,
			errorOnDelete: errors.New("wanted error"),
			expectActions: []kubetesting.Action{
				newNodeDeleteAction(testNode.Name),
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
			mockClient.AddReactor("delete", "nodes", func(action kubetesting.Action) (bool, runtime.Object, error) {
				return true, nil, test.errorOnDelete
			})

			u.ctx.K8sClient = mockClient
			err := u.DeleteNode(test.node)

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

func TestHandleNodeFullList(t *testing.T) {
	fullList := []string{"node1"}

	ops := metav1.ListOptions{
		LabelSelector: "ote-cluster=c1",
	}

	cmNodeList := &corev1.NodeList{}
	node := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node1-c1",
			Labels: map[string]string{reporter.ClusterLabel: "c1"},
		},
	}
	cmNodeList.Items = append(cmNodeList.Items, node)

	cmNodeList2 := &corev1.NodeList{}
	node = corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node2-c1",
			Labels: map[string]string{reporter.ClusterLabel: "c1"},
		},
	}
	cmNodeList2.Items = append(cmNodeList2.Items, node)

	tests := []struct {
		name          string
		clusterName   string
		edgeNodeList  []string
		nodeList      *corev1.NodeList
		expectActions []kubetesting.Action
		expectErr     bool
	}{
		{
			name:         "success to handle full list's node",
			clusterName:  "c1",
			edgeNodeList: fullList,
			nodeList:     cmNodeList,
			expectActions: []kubetesting.Action{
				newNodeListAction(ops),
			},
			expectErr: false,
		},
		{
			name:         "A error occurs when handles a full list node",
			clusterName:  "c1",
			edgeNodeList: fullList,
			nodeList:     cmNodeList2,
			expectActions: []kubetesting.Action{
				newNodeListAction(ops),
				newNodeDeleteAction("node2"),
			},
			expectErr: true,
		},
	}

	for _, test := range tests {
		mockClient := &kubernetes.Clientset{}
		mockClient.AddReactor("list", "nodes", func(action kubetesting.Action) (bool, runtime.Object, error) {
			return true, test.nodeList, nil
		})

		u := NewUpstreamProcessor(&K8sContext{})
		u.ctx.K8sClient = mockClient

		u.handleNodeFullList(test.clusterName, test.edgeNodeList)

		if test.expectErr {
			assert.NotEqual(t, test.expectActions, mockClient.Actions())
		} else {
			assert.Equal(t, test.expectActions, mockClient.Actions())
		}
	}
}
