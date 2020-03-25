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
	"fmt"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/clustermessage"
	"github.com/baidu/ote-stack/pkg/k8sclient"
	"github.com/baidu/ote-stack/pkg/reporter"
)

const (
	UniqueResourceNameSeparator = "-"
)

var noGracePeriodSeconds int64

// UpstreamProcessor processes msg from root cluster controller.
type UpstreamProcessor struct {
	ctx        *K8sContext
	clusterCRD *k8sclient.ClusterCRD
}

// NewUpstreamProcessor new a UpstreamProcessor with k8s context.
func NewUpstreamProcessor(ctx *K8sContext) *UpstreamProcessor {
	return &UpstreamProcessor{
		ctx:        ctx,
		clusterCRD: k8sclient.NewClusterCRD(ctx.OteClient),
	}
}

// HandleReceivedMessage processes msg from root cluster controller.
// This function should be registed to controller tunnel.
func (u *UpstreamProcessor) HandleReceivedMessage(client string, data []byte) (ret error) {
	// get ClusterMessage from data
	msg := &clustermessage.ClusterMessage{}
	err := msg.Deserialize(data)
	if err != nil {
		ret = fmt.Errorf("handleReceivedMessage failed %v", err)
		klog.Errorf("%v", ret)
		return
	}

	if msg.Head == nil {
		ret = fmt.Errorf("handleReceivedMessage failed: message head is nil")
		klog.Error(ret)
		return
	}

	// TODO add other command cases
	switch msg.Head.Command {
	case clustermessage.CommandType_EdgeReport:
		ret = u.processEdgeReport(msg)
		if ret != nil {
			klog.Errorf("processEdgeReport failed: %v", ret)
		}
	default:
		ret = fmt.Errorf("handleReceivedMessage failed: %s command not supported", msg.Head.Command.String())
		klog.Error(ret)
	}
	return
}

func (u *UpstreamProcessor) processEdgeReport(msg *clustermessage.ClusterMessage) (err error) {
	klog.V(3).Info("start processEdgeReport")

	reports, err := ReportDeserialize(msg.Body)
	if err != nil {
		klog.Errorf("deserialize reports(%s) failed: %v", msg.Body, err)
		return
	}

	//TODO:more resource handle.
	for _, report := range reports {
		switch report.ResourceType {
		case reporter.ResourceTypePod:
			if err = u.handlePodReport(msg.Head.ClusterName, report.Body); err != nil {
				klog.Errorf("handlePodReport failed: %v", err)
			}
		case reporter.ResourceTypeNode:
			if err = u.handleNodeReport(msg.Head.ClusterName, report.Body); err != nil {
				klog.Errorf("handleNodeReport failed: %v", err)
			}
		case reporter.ResourceTypeClusterStatus:
			if err = u.handleClusterStatusReport(msg.Head.ClusterName, report.Body); err != nil {
				klog.Errorf("handleClusterStatusReport failed: %v", err)
			}
		case reporter.ResourceTypeDeployment:
			if err = u.handleDeploymentReport(msg.Head.ClusterName, report.Body); err != nil {
				klog.Errorf("handleDeploymentReport failed: %v", err)
			}
		case reporter.ResourceTypeDaemonset:
			if err = u.handleDaemonsetReport(msg.Head.ClusterName, report.Body); err != nil {
				klog.Errorf("handleDaemonsetReport failed: %v", err)
			}
		case reporter.ResourceTypeService:
			if err = u.handleServiceReport(msg.Head.ClusterName, report.Body); err != nil {
				klog.Errorf("handleServiceReport failed: %v", err)
			}
		case reporter.ResourceTypeEvent:
			if err = u.handleEventReport(report.Body); err != nil {
				klog.Errorf("handleEventReport failed: %v", err)
			}
		default:
			klog.Errorf("processEdgeReport failed, reource type(%d) not support", report.ResourceType)
		}
	}
	return
}

// ReportDeserialize deserialize byte data to report slice.
func ReportDeserialize(b []byte) ([]reporter.Report, error) {
	reports := []reporter.Report{}
	err := json.Unmarshal(b, &reports)
	if err != nil {
		return nil, err
	}
	return reports, nil
}

// UniqueResourceName returns unique resource name.
func UniqueResourceName(obj *metav1.ObjectMeta) error {
	if obj.Labels[reporter.ClusterLabel] == "" {
		return fmt.Errorf("ClusterLabel is empty")
	}
	obj.Name = obj.Name + UniqueResourceNameSeparator + obj.Labels[reporter.ClusterLabel]

	return nil
}

// checkEdgeVersion checks if resource reported from edge cluster is newer than the one stored in etcd.
func checkEdgeVersion(reportResource *metav1.ObjectMeta, storedResource *metav1.ObjectMeta) bool {
	if reportResource.Labels == nil || storedResource.Labels == nil {
		klog.Errorf("resource's labels is empty")
		return false
	}

	if reportResource.Labels[reporter.EdgeVersionLabel] == "" || storedResource.Labels[reporter.EdgeVersionLabel] == "" {
		klog.Errorf("resource's edge-version is empty")
		return false
	}

	edgeVersion, err := strconv.Atoi(reportResource.Labels[reporter.EdgeVersionLabel])
	if err != nil {
		klog.Errorf("check edge version failed: %v", err)
		return false
	}

	storedVersion, err := strconv.Atoi(storedResource.Labels[reporter.EdgeVersionLabel])
	if err != nil {
		klog.Errorf("check edge version failed: %v", err)
		return false
	}

	//resource report sequential checking
	if edgeVersion < storedVersion {
		klog.Errorf("Current resource's edge-version(%s) is less than or equal to ETCD's resource's edge-version(%s)",
			reportResource.Labels[reporter.EdgeVersionLabel], storedResource.Labels[reporter.EdgeVersionLabel])
		return false
	}

	return true
}

// adaptToCentralResource adapts the reported resource to the one stored in center etcd before updating it.
func adaptToCentralResource(reportResource *metav1.ObjectMeta, storedResource *metav1.ObjectMeta) {
	// The resource updated to etcd should have the same ResourceVersion of the one stored in etcd.
	reportResource.ResourceVersion = storedResource.ResourceVersion

	// The resource from edge cluster should have the same uid of the one stored in etcd.
	reportResource.UID = storedResource.UID
}

// UniqueFullResourceName returns unique resource name.
func UniqueFullResourceName(name string, clusterName string) string {
	return name + UniqueResourceNameSeparator + clusterName
}
