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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubescheme "k8s.io/client-go/kubernetes/scheme"
	ref "k8s.io/client-go/tools/reference"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/reporter"
)

// handleEventReport handles EventReport from edge clusters.
func (u *UpstreamProcessor) handleEventReport(b []byte) error {
	ers, err := EventReportStatusDeserialize(b)
	if err != nil {
		return fmt.Errorf("EventReportStatusDeserialize failed: %v", err)
	}

	//handle FullList
	if ers.FullList != nil {
		//TODO:handle full event resource.
	}

	//handle UpdateMap
	if ers.UpdateMap != nil {
		u.CenterCreateEvent(ers.UpdateMap)
	}

	//handle DelMap
	if ers.DelMap != nil {
		u.CenterDeleteEvent(ers.DelMap)
	}

	return nil
}

// CenterCreateEvent handles event resource created from edge clusters.
func (u *UpstreamProcessor) CenterCreateEvent(updateMap map[string]*corev1.Event) {
	for _, event := range updateMap {
		err := UniqueResourceName(&event.ObjectMeta)
		if err != nil {
			klog.Errorf("handleEventUpdateMap's UniqueResourceName method failed: %v", err)
			continue
		}

		err = u.relateToPodOrNode(event)
		if err != nil {
			klog.Errorf("Synchronization of the involved object's name failed: %v", err)
			continue
		}

		err = u.CreateEvent(event)
		if err != nil {
			klog.Errorf("event: %s created failed: %v", event.ObjectMeta.Name, err)
			continue
		}
	}
}

// CenterDeleteEvent handles event resource deleted from edge clusters.
func (u *UpstreamProcessor) CenterDeleteEvent(delMap map[string]*corev1.Event) {
	for _, event := range delMap {
		err := UniqueResourceName(&event.ObjectMeta)
		if err != nil {
			klog.Errorf("handleEventDelMap's UniqueResourceName method failed: %v", err)
			continue
		}

		err = u.DeleteEvent(event)
		if err != nil {
			klog.Errorf("event: %s deleted failed: %v", event.ObjectMeta.Name, err)
			continue
		}

		klog.V(3).Infof("Reported event resource: %s deleted success.", event.Name)
	}
}

// DeleteEvent deletes event resource reported from edge cluster.
func (u *UpstreamProcessor) DeleteEvent(event *corev1.Event) error {
	return u.ctx.K8sClient.CoreV1().Events(event.Namespace).Delete(event.Name, metav1.NewDeleteOptions(0))
}

// CreateEvent creates event resource from edge cluster to center etcd.
func (u *UpstreamProcessor) CreateEvent(event *corev1.Event) error {
	// ResourceVersion should not be set when resource is to be created.
	event.ResourceVersion = ""

	_, err := u.ctx.K8sClient.CoreV1().Events(event.Namespace).Create(event)
	if err != nil {
		return err
	}

	klog.V(3).Infof("Reported event resource: %s created success.", event.Name)
	return nil
}

// EventReportStatusDeserialize deserialize byte data to EventResourceStatus.
func EventReportStatusDeserialize(b []byte) (*reporter.EventResourceStatus, error) {
	eventReportStatus := reporter.EventResourceStatus{}

	err := json.Unmarshal(b, &eventReportStatus)
	if err != nil {
		return nil, err
	}
	return &eventReportStatus, nil
}

// relateToPodOrNode synchronizes the event-related object to the reported object,
// so that the event can be bound to the related reported object.
func (u *UpstreamProcessor) relateToPodOrNode(event *corev1.Event) error {
	if event.Labels == nil || event.Labels[reporter.ClusterLabel] == "" {
		return fmt.Errorf("event's label is null")
	}

	resourceName := event.InvolvedObject.Name + UniqueResourceNameSeparator + event.Labels[reporter.ClusterLabel]
	resourceNamespace := event.InvolvedObject.Namespace

	switch event.InvolvedObject.Kind {
	case "Pod":
		storedPod, err := u.ctx.K8sClient.CoreV1().Pods(resourceNamespace).Get(resourceName, metav1.GetOptions{})
		if err != nil {
			// TODO: if the corresponding pod do not exist, put the event into a channel to process later
			return err
		}

		obj, err := ref.GetReference(kubescheme.Scheme, storedPod)
		if err != nil {
			return err
		}

		if _, isMirrorPod := storedPod.Annotations[corev1.MirrorPodAnnotationKey]; isMirrorPod {
			obj.UID = types.UID(storedPod.Annotations[corev1.MirrorPodAnnotationKey])
		}

		event.InvolvedObject = *obj
	case "Node":
		storedNode, err := u.ctx.K8sClient.CoreV1().Nodes().Get(resourceName, metav1.GetOptions{})
		if err != nil {
			// TODO: if the corresponding node do not exist, put the event into a channel to process later
			return err
		}

		obj, err := ref.GetReference(kubescheme.Scheme, storedNode)
		if err != nil {
			return err
		}
		obj.UID = types.UID(obj.Name)

		event.InvolvedObject = *obj
	default:
		return fmt.Errorf("resource type of the event-involved object is not supported")
	}

	return nil
}
