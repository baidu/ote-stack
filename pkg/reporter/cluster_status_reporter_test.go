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

package reporter

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/baidu/ote-stack/pkg/clustermessage"
)

func TestIsNodeReady(t *testing.T) {
	testcase := []struct {
		Name         string
		Node         *corev1.Node
		ExpectResult bool
	}{
		{
			Name: "unschedulable node",
			Node: &corev1.Node{
				Spec: corev1.NodeSpec{
					Unschedulable: true,
				},
			},
			ExpectResult: false,
		},
		{
			Name: "ready node",
			Node: &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:   corev1.NodeReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			ExpectResult: true,
		},
		{
			Name: "not ready node",
			Node: &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:   corev1.NodeReady,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			ExpectResult: false,
		},
		{
			Name:         "unknown node",
			Node:         &corev1.Node{},
			ExpectResult: false,
		},
	}

	for _, tc := range testcase {
		t.Run(tc.Name, func(t *testing.T) {
			result := isNodeReady(tc.Node)
			assert.Equal(t, tc.ExpectResult, result)
		})
	}
}

func newFakeNode(cpuCapcity, memCapcity, cpuAlloc, memAlloc int64, status corev1.ConditionStatus) *corev1.Node {
	node := &corev1.Node{
		Status: corev1.NodeStatus{
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    *resource.NewQuantity(cpuCapcity, resource.DecimalSI),
				corev1.ResourceMemory: *resource.NewQuantity(memCapcity, resource.BinarySI),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    *resource.NewQuantity(cpuAlloc, resource.DecimalSI),
				corev1.ResourceMemory: *resource.NewQuantity(memAlloc, resource.BinarySI),
			},
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: status,
				},
			},
		},
	}

	return node
}

func TestCaculateClusterResource(t *testing.T) {
	testcase := []struct {
		Name         string
		Nodes        *corev1.NodeList
		ExpectResult *ClusterResource
	}{
		{
			Name: "all ready node",
			Nodes: &corev1.NodeList{
				Items: []corev1.Node{
					*newFakeNode(16, 1024, 12, 512, corev1.ConditionTrue),
					*newFakeNode(16, 1024, 12, 512, corev1.ConditionTrue),
				},
			},
			ExpectResult: &ClusterResource{
				Capacity: map[corev1.ResourceName]*resource.Quantity{
					corev1.ResourceCPU:    resource.NewQuantity(32, resource.DecimalSI),
					corev1.ResourceMemory: resource.NewQuantity(2048, resource.BinarySI),
				},
				Allocatable: map[corev1.ResourceName]*resource.Quantity{
					corev1.ResourceCPU:    resource.NewQuantity(24, resource.DecimalSI),
					corev1.ResourceMemory: resource.NewQuantity(1024, resource.BinarySI),
				},
			},
		},
		{
			Name: "exist a notready node",
			Nodes: &corev1.NodeList{
				Items: []corev1.Node{
					*newFakeNode(16, 1024, 12, 512, corev1.ConditionTrue),
					*newFakeNode(16, 1024, 12, 512, corev1.ConditionTrue),
					*newFakeNode(16, 1024, 12, 512, corev1.ConditionFalse),
				},
			},
			ExpectResult: &ClusterResource{
				Capacity: map[corev1.ResourceName]*resource.Quantity{
					corev1.ResourceCPU:    resource.NewQuantity(32, resource.DecimalSI),
					corev1.ResourceMemory: resource.NewQuantity(2048, resource.BinarySI),
				},
				Allocatable: map[corev1.ResourceName]*resource.Quantity{
					corev1.ResourceCPU:    resource.NewQuantity(24, resource.DecimalSI),
					corev1.ResourceMemory: resource.NewQuantity(1024, resource.BinarySI),
				},
			},
		},
	}

	for _, tc := range testcase {
		t.Run(tc.Name, func(t *testing.T) {
			result := caculateClusterResource(tc.Nodes)
			assert.Equal(t, tc.ExpectResult, result)
		})
	}
}

func TestNewClusterStatusReporter(t *testing.T) {
	testcase := []struct {
		Name        string
		Context     *ReporterContext
		ExpectError bool
	}{
		{
			Name: "syncChan is nil",
			Context: &ReporterContext{
				SyncChan: nil,
			},
			ExpectError: true,
		},
		{
			Name: "kubeClient is nil",
			Context: &ReporterContext{
				SyncChan:        make(chan clustermessage.ClusterMessage),
				KubeClient:      nil,
				InformerFactory: informers.NewSharedInformerFactory(fake.NewSimpleClientset(), 1*time.Second),
				StopChan:        make(chan struct{}),
			},
			ExpectError: true,
		},
		{
			Name: "valid context",
			Context: &ReporterContext{
				SyncChan:        make(chan clustermessage.ClusterMessage),
				KubeClient:      fake.NewSimpleClientset(),
				InformerFactory: informers.NewSharedInformerFactory(fake.NewSimpleClientset(), 1*time.Second),
				StopChan:        make(chan struct{}),
			},
			ExpectError: false,
		},
	}
	for _, tc := range testcase {
		t.Run(tc.Name, func(t *testing.T) {
			_, err := newClusterStatusReporter(tc.Context)
			if tc.ExpectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSyncClusterStatus(t *testing.T) {
	expectClusterName := "test-cluster"
	expectNode := newFakeNode(16, 2048, 12, 1024, corev1.ConditionTrue)
	client := fake.NewSimpleClientset(expectNode)

	ctx := &ReporterContext{
		ClusterName:     func() string { return expectClusterName },
		SyncChan:        make(chan clustermessage.ClusterMessage, 1),
		KubeClient:      client,
		InformerFactory: informers.NewSharedInformerFactory(client, 1*time.Second),
		StopChan:        make(chan struct{}, 1),
	}

	report, err := newClusterStatusReporter(ctx)
	assert.NoError(t, err)

	report.syncClusterStatus()

	var msg clustermessage.ClusterMessage

	select {
	case <-time.After(3 * time.Second):
		assert.Failf(t, "SynChan receive msg timeout", "")
		return
	case msg = <-ctx.SyncChan:
	}

	assert.Equal(t, msg.Head.Command, clustermessage.CommandType_EdgeReport)
	assert.Equal(t, msg.Head.ClusterName, expectClusterName)

	result := []Report{}
	err = json.Unmarshal(msg.Body, &result)
	assert.NoError(t, err)
	assert.Len(t, result, 1)

	status := ClusterStatus{}
	err = json.Unmarshal(result[0].Body, &status)
	assert.NoError(t, err)

	for name, quantity := range expectNode.Status.Capacity {
		value, exist := status.Capacity[name]
		if assert.True(t, exist, "Capacity resource %s should be existed", name) {
			assert.Zero(t, quantity.Cmp(*value), "Capacity resource %s should be equal", name)
		}
	}

	for name, quantity := range expectNode.Status.Allocatable {
		value, exist := status.Allocatable[name]
		if assert.True(t, exist, "Allocatable resource %s should be existed", name) {
			assert.Zero(t, quantity.Cmp(*value), "Allocatable resource %s should be equal", name)
		}
	}
}
