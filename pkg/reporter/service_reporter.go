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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/clustermessage"
)

type ServiceReporter struct {
	SyncChan chan clustermessage.ClusterMessage
	ctx      *ReporterContext
}

// startServiceReporter inits service reporter and starts to watch service resource.
func startServiceReporter(ctx *ReporterContext) error {
	if !ctx.IsValid() {
		return fmt.Errorf("ReporterContext validation failed")
	}

	serviceReporter := &ServiceReporter{
		ctx:      ctx,
		SyncChan: ctx.SyncChan,
	}

	// Regists EventHandler for service informer listing and watching service resource.
	ctx.InformerFactory.Core().V1().Services().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: serviceReporter.handleService,
		UpdateFunc: func(old, new interface{}) {
			newService := new.(*corev1.Service)
			oldService := old.(*corev1.Service)
			if newService.ResourceVersion == oldService.ResourceVersion {
				// Periodic resync will send update events for all known Service.
				// Two different versions of the same Service will always have different RVs.
				return
			}
			serviceReporter.handleService(new)
		},
		DeleteFunc: serviceReporter.deleteService,
	})

	return nil
}

// handleService is used to handle the creation and update operations of the service.
func (sr *ServiceReporter) handleService(obj interface{}) {
	service, ok := obj.(*corev1.Service)
	if !ok {
		klog.Errorf("Should be Service object but encounter others in handleService.")
		return
	}
	klog.V(3).Infof("handle Service: %s", service.Name)

	addLabelToResource(&service.ObjectMeta, sr.ctx)

	if sr.ctx.IsLightweightReport {
		service = sr.lightWeightService(service)
	}

	// generates unique key for service.
	key, err := cache.MetaNamespaceKeyFunc(service)
	if err != nil {
		klog.Errorf("Failed to get map key: %s", err)
		return
	}

	serviceMap := &ServiceResourceStatus{
		UpdateMap: map[string]*corev1.Service{
			key: service,
		},
	}

	go sr.sendToSyncChan(serviceMap)
}

// deleteService is used to handle the removal of the service.
func (sr *ServiceReporter) deleteService(obj interface{}) {
	service, ok := obj.(*corev1.Service)
	if !ok {
		klog.Errorf("Should be Service object but encounter others in deleteService")
		return
	}
	klog.V(3).Infof("Service: %s deleted.", service.Name)

	addLabelToResource(&service.ObjectMeta, sr.ctx)

	if sr.ctx.IsLightweightReport {
		service = sr.lightWeightService(service)
	}

	// generates unique key for service.
	key, err := cache.MetaNamespaceKeyFunc(service)
	if err != nil {
		klog.Errorf("Failed to get map key: %s", err)
		return
	}

	serviceMap := &ServiceResourceStatus{
		DelMap: map[string]*corev1.Service{
			key: service,
		},
	}

	go sr.sendToSyncChan(serviceMap)
}

// sendToSyncChan sends wrapped ClusterMessage data to SyncChan.
func (sr *ServiceReporter) sendToSyncChan(serviceMap *ServiceResourceStatus) {
	serviceReports, err := serviceMap.serializeMapToReporters()
	if err != nil {
		klog.Errorf("serialize map failed: %v", err)
		return
	}

	msg, err := serviceReports.ToClusterMessage(sr.ctx.ClusterName())
	if err != nil {
		klog.Errorf("change service Reports to clustermessage failed: %v", err)
		return
	}

	sr.SyncChan <- *msg
}

// serializeMapToReporters serializes ServiceResourceStatus and converts to Reports.
func (sr *ServiceResourceStatus) serializeMapToReporters() (Reports, error) {
	serviceJson, err := json.Marshal(sr)
	if err != nil {
		return nil, err
	}

	data := Reports{
		{
			ResourceType: ResourceTypeService,
			Body:         serviceJson,
		},
	}

	return data, nil
}

// lightWeightService crops the content of the service
func (sr *ServiceReporter) lightWeightService(service *corev1.Service) *corev1.Service {
	return &corev1.Service{
		TypeMeta: service.TypeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name:      service.Name,
			Namespace: service.Namespace,
			Labels:    service.Labels,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: service.Spec.ClusterIP,
			Ports:     service.Spec.Ports,
		},
	}
}
