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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/clustermessage"
)

// NodeReporter is responsible for synchronizing node status of edge cluster.
type NodeReporter struct {
	SyncChan chan clustermessage.ClusterMessage

	ctx *ReporterContext
}

// newNodeReporter creates a new NodeReporter.
func newNodeReporter(ctx *ReporterContext) (*NodeReporter, error) {
	if !ctx.IsValid() {
		return nil, fmt.Errorf("ReporterContext validation failed")
	}

	nodeReporter := &NodeReporter{
		ctx:      ctx,
		SyncChan: ctx.SyncChan,
	}

	ctx.InformerFactory.Core().V1().Nodes().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: nodeReporter.handleNode,
		UpdateFunc: func(old, new interface{}) {
			newPod := new.(*corev1.Node)
			oldPod := old.(*corev1.Node)
			if newPod.ResourceVersion == oldPod.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				return
			}
			nodeReporter.handleNode(new)
		},
		DeleteFunc: nodeReporter.deleteNode,
	})

	return nodeReporter, nil
}

func startNodeReporter(ctx *ReporterContext) error {
	_, err := newNodeReporter(ctx)
	if err != nil {
		klog.Errorf("Failed to start node reporter: %v", err)
		return err
	}

	return nil
}

// handleNode is used to handle the creation and update operations of the node.
func (nr *NodeReporter) handleNode(obj interface{}) {
	node, ok := obj.(*corev1.Node)
	if !ok {
		klog.Errorf("Should be Node object but encounter others in handleNode")

		return
	}

	// k8s labels may be nil，need to make it
	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}

	node.Labels[ClusterLabel] = nr.ctx.ClusterName()
	// support for CM sequential checking
	node.Labels[EdgeVersionLabel] = node.ResourceVersion

	key, err := cache.MetaNamespaceKeyFunc(node)
	if err != nil {
		klog.Errorf("Failed to get map key: %s", err)
		return
	}

	klog.V(3).Infof("find node : %s,node: %v", key, node)

	// adds node objects to UpdateMap.
	nodeMap := &NodeResourceStatus{
		UpdateMap: map[string]*corev1.Node{
			key: node,
		},
	}

	go nr.sendToSyncChan(nodeMap)
}

func (nr *NodeReporter) deleteNode(obj interface{}) {
	node, ok := obj.(*corev1.Node)
	if !ok {
		klog.Errorf("Should be Node object but encounter others in deleteNode")
		return
	}
	// k8s labels may be nil，need to make it
	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}

	node.Labels[ClusterLabel] = nr.ctx.ClusterName()
	// support for CM sequential checking
	node.Labels[EdgeVersionLabel] = node.ResourceVersion

	key, err := cache.MetaNamespaceKeyFunc(node)
	if err != nil {
		klog.Errorf("Failed to get map key: %v", err)
		return
	}

	// adds node objects to DelMap.
	nodeMap := &NodeResourceStatus{
		DelMap: map[string]*corev1.Node{
			key: node,
		},
	}

	go nr.sendToSyncChan(nodeMap)
}

func (nr *NodeReporter) sendToSyncChan(nodeMap *NodeResourceStatus) {
	nodeReports, err := nodeMap.serializeMapToReports()
	if err != nil {
		klog.Errorf("serialize map failed: %v", err)
		return
	}

	msg, err := nodeReports.ToClusterMessage(nr.ctx.ClusterName())
	if err != nil {
		klog.Errorf("change node Reports to ClusterMessage failed: %v", err)
		return
	}

	nr.SyncChan <- *msg
}

// serializeMapToJSON define node report content and convert to json.
func (rs *NodeResourceStatus) serializeMapToReports() (Reports, error) {
	mapJSON, err := json.Marshal(*rs)
	if err != nil {
		klog.Errorf("serialize map failed: %v", err)
		return nil, err
	}

	data := Reports{
		{
			ResourceType: ResourceTypeNode,
			Body:         mapJSON,
		},
	}

	return data, nil
}
