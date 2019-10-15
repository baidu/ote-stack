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
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kubernetes "k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/clustermessage"
)

const (
	clusterStatusSyncPeriod = 120 * time.Second
)

// ClusterStatusReporter is responsible for synchronizing information about the status of a cluster.
type ClusterStatusReporter struct {
	syncChan    chan clustermessage.ClusterMessage
	kubeClient  kubernetes.Interface
	clusterName func() string
}

func startClusterStatusReporter(ctx *ReporterContext) error {
	reporter, err := newClusterStatusReporter(ctx)
	if err != nil {
		return err
	}

	go reporter.Run(ctx.StopChan)

	return nil
}

func newClusterStatusReporter(ctx *ReporterContext) (*ClusterStatusReporter, error) {
	if !ctx.IsValid() {
		return nil, fmt.Errorf("ReporterContext validation failed")
	}

	if ctx.KubeClient == nil {
		return nil, fmt.Errorf("kubeclient is nil")
	}

	return &ClusterStatusReporter{
		syncChan:    ctx.SyncChan,
		kubeClient:  ctx.KubeClient,
		clusterName: ctx.ClusterName,
	}, nil
}

// Run starts a cron job that synchronizes information of the cluster.
func (c *ClusterStatusReporter) Run(stopCh <-chan struct{}) {
	klog.Infof("Starting cluster status reporter")
	defer klog.Infof("Shutting down cluster status reporter")

	go wait.Until(c.syncClusterStatus, clusterStatusSyncPeriod, stopCh)

	<-stopCh
}

func (c *ClusterStatusReporter) syncClusterStatus() {
	list, err := c.kubeClient.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		klog.Errorf("can not list node: %v", err)
		return
	}

	status := &ClusterStatus{
		ClusterResource: *caculateClusterResource(list),
	}

	clusterStatusJSON, err := json.Marshal(status)
	if err != nil {
		klog.Errorf("serialize cluster status failed: %v", err)
		return
	}

	data := Reports{
		{
			ResourceType: ResourceTypeClusterStatus,
			Body:         clusterStatusJSON,
		},
	}

	msg, err := data.ToClusterMessage(c.clusterName())
	if err != nil {
		klog.Errorf("change Reports to Clustermessage failed: %v", err)
		return
	}

	c.syncChan <- *msg
}

func isNodeReady(node *corev1.Node) bool {
	if node.Spec.Unschedulable {
		return false
	}

	for i := range node.Status.Conditions {
		if node.Status.Conditions[i].Type == corev1.NodeReady {
			if node.Status.Conditions[i].Status == corev1.ConditionTrue {
				return true
			}
			return false
		}
	}

	return false
}

func caculateClusterResource(nodes *corev1.NodeList) *ClusterResource {
	clusterResource := &ClusterResource{
		Capacity:    make(map[corev1.ResourceName]*resource.Quantity),
		Allocatable: make(map[corev1.ResourceName]*resource.Quantity),
	}

	for _, node := range nodes.Items {
		if !isNodeReady(&node) {
			continue
		}

		for name, value := range node.Status.Allocatable {
			if _, exist := clusterResource.Allocatable[name]; !exist {
				clusterResource.Allocatable[name] = &resource.Quantity{}
			}
			clusterResource.Allocatable[name].Add(value)
		}
		for name, value := range node.Status.Capacity {
			if _, exist := clusterResource.Capacity[name]; !exist {
				clusterResource.Capacity[name] = &resource.Quantity{}
			}
			clusterResource.Capacity[name].Add(value)
		}
	}
	return clusterResource
}
