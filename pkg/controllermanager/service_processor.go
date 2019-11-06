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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/reporter"
)

// handleServiceReport handles ServiceReport from edge clusters.
func (u *UpstreamProcessor) handleServiceReport(b []byte) error {
	srs, err := ServiceReportStatusDeserialize(b)
	if err != nil {
		return fmt.Errorf("ServiceReportStatusDeserialize failed: %v", err)
	}

	//handle FullList
	if srs.FullList != nil {
		//TODO:handle full service resource.
	}

	//handle UpdateMap
	if srs.UpdateMap != nil {
		u.handleServiceUpdateMap(srs.UpdateMap)
	}

	//handle DelMap
	if srs.DelMap != nil {
		u.handleServiceDelMap(srs.DelMap)
	}

	return nil
}

// handleServiceUpdateMap handles service resource created or updated event from edge clusters.
func (u *UpstreamProcessor) handleServiceUpdateMap(updateMap map[string]*corev1.Service) {
	for _, service := range updateMap {
		err := UniqueResourceName(&service.ObjectMeta)
		if err != nil {
			klog.Errorf("handleServiceUpdateMap's UniqueResourceName method failed: %v", err)
			continue
		}

		err = u.CreateOrUpdateService(service)
		if err != nil {
			klog.Errorf("service: %s created or updated failed: %v", service.ObjectMeta.Name, err)
			continue
		}
	}
}

// handleServiceDelMap handles service resource deleted event from edge clusters.
func (u *UpstreamProcessor) handleServiceDelMap(delMap map[string]*corev1.Service) {
	for _, service := range delMap {
		err := UniqueResourceName(&service.ObjectMeta)
		if err != nil {
			klog.Errorf("handleServiceDelMap's UniqueResourceName method failed: %v", err)
			continue
		}

		err = u.DeleteService(service)
		if err != nil {
			klog.Errorf("service: %s deleted failed: %v", service.ObjectMeta.Name, err)
			continue
		}

		klog.V(3).Infof("Reported service resource: %s deleted success.", service.Name)
	}
}

// CreateOrUpdateService checks whether service exists,then creates or updates it to center etcd.
func (u *UpstreamProcessor) CreateOrUpdateService(service *corev1.Service) error {
	_, err := u.GetService(service)
	// If not found resource, creates it.
	if err != nil && errors.IsNotFound(err) {
		err = u.CreateService(service)
	}

	if err != nil {
		return err
	}

	// If service exists, updates it.
	return u.UpdateService(service)
}

// DeleteService deletes service resource reported from edge cluster.
func (u *UpstreamProcessor) DeleteService(service *corev1.Service) error {
	return u.ctx.K8sClient.CoreV1().Services(service.Namespace).Delete(service.Name, metav1.NewDeleteOptions(0))
}

// GetService get service resource stored in center etcd.
func (u *UpstreamProcessor) GetService(service *corev1.Service) (*corev1.Service, error) {
	storedService, err := u.ctx.K8sClient.CoreV1().Services(service.Namespace).Get(service.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return storedService, nil
}

// CreateService creates service resource from edge cluster to center etcd.
func (u *UpstreamProcessor) CreateService(service *corev1.Service) error {
	// ResourceVersion should not be set when resource is to be created.
	service.ResourceVersion = ""

	_, err := u.ctx.K8sClient.CoreV1().Services(service.Namespace).Create(service)
	if err != nil {
		return err
	}

	klog.V(3).Infof("Reported service resource: %s created success.", service.Name)
	return nil
}

// UpdateService updates service resource from edge cluster to center etcd.
func (u *UpstreamProcessor) UpdateService(service *corev1.Service) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		storedService, err := u.GetService(service)
		if err != nil {
			return err
		}

		if !checkEdgeVersion(&service.ObjectMeta, &storedService.ObjectMeta) {
			return fmt.Errorf("check service edge version failed")
		}

		adaptToCentralResource(&service.ObjectMeta, &storedService.ObjectMeta)

		_, err = u.ctx.K8sClient.CoreV1().Services(service.Namespace).Update(service)
		return err
	})

	if err != nil {
		return err
	}

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		storedService, err := u.GetService(service)
		if err != nil {
			return err
		}

		if !checkEdgeVersion(&service.ObjectMeta, &storedService.ObjectMeta) {
			return fmt.Errorf("check service edge version failed")
		}

		adaptToCentralResource(&service.ObjectMeta, &storedService.ObjectMeta)

		_, err = u.ctx.K8sClient.CoreV1().Services(service.Namespace).UpdateStatus(service)
		return err
	})

	if err != nil {
		return err
	}

	klog.V(3).Infof("Reported service resource: %s updated success.", service.Name)

	return nil
}

// ServiceReportStatusDeserialize deserialize byte data to ServiceResourceStatus.
func ServiceReportStatusDeserialize(b []byte) (*reporter.ServiceResourceStatus, error) {
	serviceReportStatus := reporter.ServiceResourceStatus{}

	err := json.Unmarshal(b, &serviceReportStatus)
	if err != nil {
		return nil, err
	}
	return &serviceReportStatus, nil
}
