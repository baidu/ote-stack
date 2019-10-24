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

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/reporter"
)

// handleDaemonsetReport handles DaemonsetReport from edge clusters.
func (u *UpstreamProcessor) handleDaemonsetReport(b []byte) error {
	drs, err := DaemonsetReportStatusDeserialize(b)
	if err != nil {
		return fmt.Errorf("DaemonsetReportStatusDeserialize failed: %v", err)
	}

	//handle FullList
	if drs.FullList != nil {
		//TODO:handle full daemonset resource.
	}

	//handle UpdateMap
	if drs.UpdateMap != nil {
		u.handleDaemonsetUpdateMap(drs.UpdateMap)
	}

	//handle DelMap
	if drs.DelMap != nil {
		u.handleDaemonsetDelMap(drs.DelMap)
	}

	return nil
}

// handleDaemonsetUpdateMap handles daemonset resource created or updated event from edge clusters.
func (u *UpstreamProcessor) handleDaemonsetUpdateMap(updateMap map[string]*appsv1.DaemonSet) {
	for _, daemonset := range updateMap {
		err := UniqueResourceName(&daemonset.ObjectMeta)
		if err != nil {
			klog.Errorf("handleDaemonsetUpdateMap's UniqueResourceName method failed: %v", err)
			continue
		}

		err = u.CreateOrUpdateDaemonset(daemonset)
		if err != nil {
			klog.Errorf("daemonset: %s created or updated failed: %v", daemonset.ObjectMeta.Name, err)
			continue
		}
	}
}

// handleDaemonsetDelMap handles daemonset resource deleted event from edge clusters.
func (u *UpstreamProcessor) handleDaemonsetDelMap(delMap map[string]*appsv1.DaemonSet) {
	for _, daemonset := range delMap {
		err := UniqueResourceName(&daemonset.ObjectMeta)
		if err != nil {
			klog.Errorf("handleDaemonsetDelMap's UniqueResourceName method failed: %v", err)
			continue
		}

		err = u.DeleteDaemonset(daemonset)
		if err != nil {
			klog.Errorf("daemonset: %s deleted failed: %v", daemonset.ObjectMeta.Name, err)
			continue
		}

		klog.V(3).Infof("Reported daemonset resource: %s deleted success.", daemonset.Name)
	}
}

// CreateOrUpdateDaemonset checks whether daemonset exists,then creates or updates it to center etcd.
func (u *UpstreamProcessor) CreateOrUpdateDaemonset(daemonset *appsv1.DaemonSet) error {
	_, err := u.GetDaemonset(daemonset)
	// If not found resource, creates it.
	if err != nil && errors.IsNotFound(err) {
		return u.CreateDaemonset(daemonset)
	}

	if err != nil {
		return err
	}

	// If daemonset exists, updates it.
	return u.UpdateDaemonset(daemonset)
}

// DeleteDaemonset deletes daemonset resource reported from edge cluster.
func (u *UpstreamProcessor) DeleteDaemonset(daemonset *appsv1.DaemonSet) error {
	return u.ctx.K8sClient.AppsV1().DaemonSets(daemonset.Namespace).Delete(daemonset.Name, metav1.NewDeleteOptions(0))
}

// GetDaemonset get daemonset resource stored in center etcd.
func (u *UpstreamProcessor) GetDaemonset(daemonset *appsv1.DaemonSet) (*appsv1.DaemonSet, error) {
	storedDaemonset, err := u.ctx.K8sClient.AppsV1().DaemonSets(daemonset.Namespace).Get(daemonset.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return storedDaemonset, nil
}

// CreateDaemonset creates daemonset resource from edge cluster to center etcd.
func (u *UpstreamProcessor) CreateDaemonset(daemonset *appsv1.DaemonSet) error {
	// ResourceVersion should not be set when resource is to be created.
	daemonset.ResourceVersion = ""

	_, err := u.ctx.K8sClient.AppsV1().DaemonSets(daemonset.Namespace).Create(daemonset)
	if err != nil {
		return err
	}

	klog.V(3).Infof("Reported daemonset resource: %s created success.", daemonset.Name)
	return nil
}

// UpdateDaemonset updates daemonset resource from edge cluster to center etcd.
func (u *UpstreamProcessor) UpdateDaemonset(daemonset *appsv1.DaemonSet) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		storedDaemonset, err := u.GetDaemonset(daemonset)
		if err != nil {
			return err
		}

		if !checkEdgeVersion(&daemonset.ObjectMeta, &storedDaemonset.ObjectMeta) {
			return fmt.Errorf("check daemonset edge version failed")
		}

		adaptToCentralResource(&daemonset.ObjectMeta, &storedDaemonset.ObjectMeta)

		_, err = u.ctx.K8sClient.AppsV1().DaemonSets(daemonset.Namespace).Update(daemonset)
		return err
	})

	if err != nil {
		return err
	}

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		storedDaemonset, err := u.GetDaemonset(daemonset)
		if err != nil {
			return err
		}

		if !checkEdgeVersion(&daemonset.ObjectMeta, &storedDaemonset.ObjectMeta) {
			return fmt.Errorf("check daemonset edge version failed")
		}

		adaptToCentralResource(&daemonset.ObjectMeta, &storedDaemonset.ObjectMeta)

		_, err = u.ctx.K8sClient.AppsV1().DaemonSets(daemonset.Namespace).UpdateStatus(daemonset)
		return err
	})

	if err != nil {
		return err
	}

	klog.V(3).Infof("Reported daemonset resource: %s updated success.", daemonset.Name)

	return nil
}

// DaemonsetReportStatusDeserialize deserialize byte data to DaemonsetResourceStatus.
func DaemonsetReportStatusDeserialize(b []byte) (*reporter.DaemonsetResourceStatus, error) {
	daemonsetReportStatus := reporter.DaemonsetResourceStatus{}

	err := json.Unmarshal(b, &daemonsetReportStatus)
	if err != nil {
		return nil, err
	}
	return &daemonsetReportStatus, nil
}
