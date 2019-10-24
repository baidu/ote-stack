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

type EventReporter struct {
	SyncChan chan clustermessage.ClusterMessage
	ctx      *ReporterContext
}

// startEventReporter inits event reporter and starts to watch events resource.
func startEventReporter(ctx *ReporterContext) error {
	if !ctx.IsValid() {
		return fmt.Errorf("ReporterContext validation failed.")
	}

	eventReporter := &EventReporter{
		ctx:      ctx,
		SyncChan: ctx.SyncChan,
	}

	// Regists EventHandler for events informer listing and watching events resource.
	ctx.InformerFactory.Core().V1().Events().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: eventReporter.handleEvent,
		UpdateFunc: func(old, new interface{}) {
			newEvent := new.(*corev1.Event)
			oldEvent := new.(*corev1.Event)
			if newEvent.ResourceVersion == oldEvent.ResourceVersion {
				// Periodic resync will send update events for all known Events resource.
				// Two different versions of the same Events will always have different RVs.
				return
			}
			eventReporter.handleEvent(new)
		},
		DeleteFunc: eventReporter.deleteEvent,
	})

	return nil
}

// handleEvent is used to handle the creation and update operations of the events resource.
func (er *EventReporter) handleEvent(obj interface{}) {
	event, ok := obj.(*corev1.Event)
	if !ok {
		klog.Errorf("Should be Event object but encounter others in handleEvent.")
		return
	}

	// Only need to report the event related to pod and node.
	if !er.isNeedToReport(event) {
		return
	}

	klog.V(3).Infof("handle Event: %s", event.Name)

	addLabelToResource(&event.ObjectMeta, er.ctx)

	// generates unique key for event.
	key, err := cache.MetaNamespaceKeyFunc(event)
	if err != nil {
		klog.Errorf("Failed to get map key: %s", err)
		return
	}

	eventMap := &EventResourceStatus{
		UpdateMap: map[string]*corev1.Event{
			key: event,
		},
	}

	go er.sendToSyncChan(eventMap)
}

// deleteEvent is used to handle the removal of the event resource.
func (er *EventReporter) deleteEvent(obj interface{}) {
	event, ok := obj.(*corev1.Event)
	if !ok {
		klog.Errorf("Should be Event object but encounter others in deleteEvent.")
		return
	}

	// Only need to report the event related to pod and node.
	if !er.isNeedToReport(event) {
		return
	}

	klog.V(3).Infof("Event: %s deleted.", event.Name)

	addLabelToResource(&event.ObjectMeta, er.ctx)

	// generates unique key for event.
	key, err := cache.MetaNamespaceKeyFunc(event)
	if err != nil {
		klog.Errorf("Failed to get map key: %s", err)
		return
	}

	eventMap := &EventResourceStatus{
		DelMap: map[string]*corev1.Event{
			key: event,
		},
	}

	go er.sendToSyncChan(eventMap)
}

// sendToSyncChan sends wrapped ClusterMessage data to SyncChan.
func (er *EventReporter) sendToSyncChan(eventMap *EventResourceStatus) {
	eventReports, err := eventMap.serializeMapToReporters()
	if err != nil {
		klog.Errorf("serialize map failed: %v", err)
		return
	}

	msg, err := eventReports.ToClusterMessage(er.ctx.ClusterName())
	if err != nil {
		klog.Errorf("change event Reports to clustermessage failed: %v", err)
		return
	}

	er.SyncChan <- *msg
}

// serializeMapToReporters serializes EventResourceStatus and converts to Reports.
func (es *EventResourceStatus) serializeMapToReporters() (Reports, error) {
	eventJson, err := json.Marshal(es)
	if err != nil {
		return nil, err
	}

	data := Reports{
		{
			ResourceType: ResourceTypeEvent,
			Body:         eventJson,
		},
	}

	return data, nil
}

// isNeedToReport check if the event needed to report.
func (er *EventReporter) isNeedToReport(event *corev1.Event) bool {
	if event.InvolvedObject.Kind != "Pod" && event.InvolvedObject.Kind != "Node" {
		return false
	}

	return true
}
