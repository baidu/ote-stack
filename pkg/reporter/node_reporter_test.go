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

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/baidu/ote-stack/pkg/clustermessage"
)

type fixtureNode struct {
	t *testing.T

	kubeclient *k8sfake.Clientset

	// Objects from here preloaded into NewSimpleFake.
	kubeobjects []runtime.Object
}

func newFixtureNode(t *testing.T) *fixtureNode {
	f := &fixtureNode{}
	f.t = t
	f.kubeobjects = []runtime.Object{}
	return f
}

func (f *fixtureNode) newNodeReporter() *NodeReporter {
	f.kubeclient = k8sfake.NewSimpleClientset(f.kubeobjects...)
	k8sI := kubeinformers.NewSharedInformerFactory(f.kubeclient, noResyncPeriodFunc())

	ctx := &ReporterContext{
		BaseReporterContext: BaseReporterContext{
			ClusterName: func() string {
				return clusterName
			},
			SyncChan: make(chan clustermessage.ClusterMessage),
			StopChan: make(chan struct{}),
		},
		InformerFactory: k8sI,
	}

	nodeReporter, err := newNodeReporter(ctx)
	assert.Nil(f.t, err)

	return nodeReporter
}

func TestNewNodeReporter(t *testing.T) {
	f := newFixtureNode(t)
	nodeReporter := f.newNodeReporter()

	assert.Equal(t, clusterName, nodeReporter.ctx.ClusterName())
}

func TestHandleNode(t *testing.T) {
	f := newFixtureNode(t)
	nodeReporter := f.newNodeReporter()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-123",
			Labels: map[string]string{ClusterLabel: clusterName},
		},
		Status: corev1.NodeStatus{Phase: corev1.NodeRunning},
	}

	// test create/update node
	nodeReporter.handleNode(node)
	data := <-nodeReporter.SyncChan
	assert.Equal(t, clustermessage.CommandType_EdgeReport, data.Head.Command)
	assert.Equal(t, clusterName, data.Head.ClusterName)

	ret := []Report{}
	err := json.Unmarshal(data.Body, &ret)
	assert.Nil(t, err)

	nrs := NodeResourceStatus{}
	err = json.Unmarshal(ret[0].Body, &nrs)
	assert.Nil(t, err)

	node = nrs.UpdateMap["node-123"]
	assert.IsType(t, &corev1.Node{}, node)
	assert.Equal(t, "node-123", node.Name)

	close(nodeReporter.SyncChan)
}

func TestDelNode(t *testing.T) {
	f := newFixtureNode(t)
	nodeReporter := f.newNodeReporter()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "del-node-123",
			Labels: map[string]string{ClusterLabel: clusterName},
		},
		Status: corev1.NodeStatus{Phase: corev1.NodeRunning},
	}

	//test delete node
	nodeReporter.deleteNode(node)

	data := <-nodeReporter.SyncChan
	assert.Equal(t, clustermessage.CommandType_EdgeReport, data.Head.Command)
	assert.Equal(t, clusterName, data.Head.ClusterName)

	ret := []Report{}
	err := json.Unmarshal(data.Body, &ret)
	assert.Nil(t, err)

	nrs := NodeResourceStatus{}
	err = json.Unmarshal(ret[0].Body, &nrs)
	assert.Nil(t, err)

	node = nrs.DelMap["del-node-123"]
	assert.IsType(t, &corev1.Node{}, node)
	assert.Equal(t, "del-node-123", node.Name)

	close(nodeReporter.SyncChan)
}
