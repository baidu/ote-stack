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
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/reporter"
)

// handleDeploymentReport handles DeploymentReport from edge clusters.
func (u *UpstreamProcessor) handleDeploymentReport(clusterName string, b []byte) error {
	drs, err := DeploymentReportStatusDeserialize(b)
	if err != nil {
		return fmt.Errorf("DeploymentReportStatusDeserialize failed: %v", err)
	}

	//handle FullList
	if drs.FullList != nil {
		u.handleDeploymentFullList(clusterName, drs.FullList)
	}

	//handle UpdateMap
	if drs.UpdateMap != nil {
		u.handleDeploymentUpdateMap(drs.UpdateMap)
	}

	//handle DelMap
	if drs.DelMap != nil {
		u.handleDeploymentDelMap(drs.DelMap)
	}

	return nil
}

// handleDeploymentFullList compares the center and edge deployment resources based on the reported full resources,
// and deletes the centers's excess deployments
func (u *UpstreamProcessor) handleDeploymentFullList(clusterName string, fullList []string) {
	var edgeDeploymentList = make(map[string]struct{})

	label := reporter.ClusterLabel + "=" + clusterName

	for _, deploymentKey := range fullList {
		edgeDeploymentList[UniqueFullResourceName(deploymentKey, clusterName)] = struct{}{}
	}

	deploymentList, err := u.ctx.K8sClient.AppsV1().Deployments("").List(metav1.ListOptions{LabelSelector: label})
	if err != nil {
		klog.Errorf("get full list deployment from cm failed")
		return
	}

	for _, deployment := range deploymentList.Items {
		// TODO Concurrent Processing Resource List
		deploymentKey, err := cache.MetaNamespaceKeyFunc(&deployment)
		if err != nil {
			klog.Errorf("get cm's deployment key failed")
			continue
		}

		if _, ok := edgeDeploymentList[deploymentKey]; !ok {
			err := u.DeleteDeployment(&deployment)
			if err != nil {
				klog.Errorf("deployment: %s deleted failed: %v", deployment.ObjectMeta.Name, err)
			}
		}
	}
}

// handleDeploymentUpdateMap handles deployment resource created or updated event from edge clusters.
func (u *UpstreamProcessor) handleDeploymentUpdateMap(updateMap map[string]*appsv1.Deployment) {
	for _, deployment := range updateMap {
		err := UniqueResourceName(&deployment.ObjectMeta)
		if err != nil {
			klog.Errorf("handleDeploymentUpdateMap's UniqueResourceName method failed: %v", err)
			continue
		}

		err = u.CreateOrUpdateDeployment(deployment)
		if err != nil {
			klog.Errorf("deployment: %s created or updated failed: %v", deployment.ObjectMeta.Name, err)
			continue
		}
	}
}

// handleDeploymentDelMap handles deployment resource deleted event from edge clusters.
func (u *UpstreamProcessor) handleDeploymentDelMap(delMap map[string]*appsv1.Deployment) {
	for _, deployment := range delMap {
		err := UniqueResourceName(&deployment.ObjectMeta)
		if err != nil {
			klog.Errorf("handleDeploymentDelMap's UniqueResourceName method failed: %v", err)
			continue
		}

		err = u.DeleteDeployment(deployment)
		if err != nil {
			klog.Errorf("deployment: %s deleted failed: %v", deployment.ObjectMeta.Name, err)
			continue
		}

		klog.V(3).Infof("Reported deployment resource: %s deleted success.", deployment.Name)
	}
}

// CreateOrUpdateDeployment checks whether deployment exists,then creates or updates it to center etcd.
func (u *UpstreamProcessor) CreateOrUpdateDeployment(deployment *appsv1.Deployment) error {
	_, err := u.GetDeployment(deployment)
	// If not found resource, creates it.
	if err != nil && errors.IsNotFound(err) {
		err = u.CreateDeployment(deployment)
	}

	if err != nil {
		return err
	}

	// If deployment exists, updates it.
	return u.UpdateDeployment(deployment)
}

// DeleteDeployment deletes deployment resource reported from edge cluster.
func (u *UpstreamProcessor) DeleteDeployment(deployment *appsv1.Deployment) error {
	return u.ctx.K8sClient.AppsV1().Deployments(deployment.Namespace).Delete(deployment.Name, metav1.NewDeleteOptions(0))
}

// GetDeployment get deployment resource stored in center etcd.
func (u *UpstreamProcessor) GetDeployment(deployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	storedDeployment, err := u.ctx.K8sClient.AppsV1().Deployments(deployment.Namespace).Get(deployment.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return storedDeployment, nil
}

// CreateDeployment creates deployment resource from edge cluster to center etcd.
func (u *UpstreamProcessor) CreateDeployment(deployment *appsv1.Deployment) error {
	// ResourceVersion should not be set when resource is to be created.
	deployment.ResourceVersion = ""

	_, err := u.ctx.K8sClient.AppsV1().Deployments(deployment.Namespace).Create(deployment)
	if err != nil {
		return err
	}

	klog.V(3).Infof("Reported deployment resource: %s created success.", deployment.Name)

	return nil
}

// UpdateDeployment updates deployment resource from edge cluster to center etcd.
func (u *UpstreamProcessor) UpdateDeployment(deployment *appsv1.Deployment) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		storedDeployment, err := u.GetDeployment(deployment)
		if err != nil {
			return err
		}

		if !checkEdgeVersion(&deployment.ObjectMeta, &storedDeployment.ObjectMeta) {
			return fmt.Errorf("check deployment edge version failed")
		}

		adaptToCentralResource(&deployment.ObjectMeta, &storedDeployment.ObjectMeta)

		_, err = u.ctx.K8sClient.AppsV1().Deployments(deployment.Namespace).Update(deployment)
		return err
	})

	if err != nil {
		return err
	}

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		storedDeployment, err := u.GetDeployment(deployment)
		if err != nil {
			return err
		}

		if !checkEdgeVersion(&deployment.ObjectMeta, &storedDeployment.ObjectMeta) {
			return fmt.Errorf("check deployment edge version failed")
		}

		adaptToCentralResource(&deployment.ObjectMeta, &storedDeployment.ObjectMeta)

		_, err = u.ctx.K8sClient.AppsV1().Deployments(deployment.Namespace).UpdateStatus(deployment)
		return err
	})

	if err != nil {
		return err
	}

	klog.V(3).Infof("Reported deployment resource: %s updated success.", deployment.Name)

	return nil
}

// DeploymentReportStatusDeserialize deserialize byte data to DeploymentResourceStatus.
func DeploymentReportStatusDeserialize(b []byte) (*reporter.DeploymentResourceStatus, error) {
	deploymentReportStatus := reporter.DeploymentResourceStatus{}

	err := json.Unmarshal(b, &deploymentReportStatus)
	if err != nil {
		return nil, err
	}
	return &deploymentReportStatus, nil
}
