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

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/clustermessage"
)

type DeploymentReporter struct {
	SyncChan chan clustermessage.ClusterMessage
	ctx      *ReporterContext
}

// startDeploymentReporter inits deployment reporter and starts to watch deployment resource.
func startDeploymentReporter(ctx *ReporterContext) error {
	if !ctx.IsValid() {
		return fmt.Errorf("ReporterContext validation failed")
	}

	deploymentReporter := &DeploymentReporter{
		ctx:      ctx,
		SyncChan: ctx.SyncChan,
	}

	// Regists EventHandler for deployment informer listing and watching deployment resource.
	// Although deployment has another API version, extensions/v1beta1，the apps/v1 version is the official stable version.
	// Just use the apps/v1 version here.
	ctx.InformerFactory.Apps().V1().Deployments().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: deploymentReporter.handleDeployment,
		UpdateFunc: func(old, new interface{}) {
			newDeployment := new.(*appsv1.Deployment)
			olddeployment := old.(*appsv1.Deployment)
			if newDeployment.ResourceVersion == olddeployment.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				return
			}
			deploymentReporter.handleDeployment(new)
		},
		DeleteFunc: deploymentReporter.deleteDeployment,
	})

	go deploymentReporter.reportFullListDeployment(ctx)

	return nil
}

// handleDeployment is used to handle the creation and update operations of the deployment.
func (dr *DeploymentReporter) handleDeployment(obj interface{}) {
	deployment, ok := obj.(*appsv1.Deployment)
	if !ok {
		klog.Errorf("Should be Deployment object but encounter others in handleDeployment.")
		return
	}
	klog.V(3).Infof("handle Deployment: %s", deployment.Name)

	addLabelToResource(&deployment.ObjectMeta, dr.ctx)

	if dr.ctx.IsLightweightReport {
		deployment = dr.lightWeightDeployment(deployment)
	}

	// generates unique key for deployment.
	key, err := cache.MetaNamespaceKeyFunc(deployment)
	if err != nil {
		klog.Errorf("Failed to get map key: %s", err)
		return
	}

	deploymentMap := &DeploymentResourceStatus{
		UpdateMap: map[string]*appsv1.Deployment{
			key: deployment,
		},
	}

	go dr.sendToSyncChan(deploymentMap)
}

// deleteDeployment is used to handle the removal of the deployment.
func (dr *DeploymentReporter) deleteDeployment(obj interface{}) {
	deployment, ok := obj.(*appsv1.Deployment)
	if !ok {
		klog.Errorf("Should be Deployment object but encounter others in deleteDeployment")
		return
	}
	klog.V(3).Infof("Deployment: %s deleted.", deployment.Name)

	addLabelToResource(&deployment.ObjectMeta, dr.ctx)

	if dr.ctx.IsLightweightReport {
		deployment = dr.lightWeightDeployment(deployment)
	}

	// generates unique key for deployment.
	key, err := cache.MetaNamespaceKeyFunc(deployment)
	if err != nil {
		klog.Errorf("Failed to get map key: %s", err)
		return
	}

	deploymentMap := &DeploymentResourceStatus{
		DelMap: map[string]*appsv1.Deployment{
			key: deployment,
		},
	}

	go dr.sendToSyncChan(deploymentMap)
}

// sendToSyncChan sends wrapped ClusterMessage data to SyncChan.
func (dr *DeploymentReporter) sendToSyncChan(deploymentMap *DeploymentResourceStatus) {
	deploymentReports, err := deploymentMap.serializeMapToReporters()
	if err != nil {
		klog.Errorf("serialize map failed: %v", err)
		return
	}

	msg, err := deploymentReports.ToClusterMessage(dr.ctx.ClusterName())
	if err != nil {
		klog.Errorf("change deployment Reports to clustermessage failed: %v", err)
		return
	}

	dr.SyncChan <- *msg
}

// serializeMapToReporters serializes DeploymentResourceStatus and converts to Reports.
func (ds *DeploymentResourceStatus) serializeMapToReporters() (Reports, error) {
	deploymentJson, err := json.Marshal(ds)
	if err != nil {
		return nil, err
	}

	data := Reports{
		{
			ResourceType: ResourceTypeDeployment,
			Body:         deploymentJson,
		},
	}

	return data, nil
}

// lightWeightDeployment crops the content of the deployment
func (dr *DeploymentReporter) lightWeightDeployment(deployment *appsv1.Deployment) *appsv1.Deployment {
	return &appsv1.Deployment{
		TypeMeta: deployment.TypeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployment.Name,
			Namespace: deployment.Namespace,
			Labels:    deployment.Labels,
		},
		Spec: appsv1.DeploymentSpec{
			Template: deployment.Spec.Template,
			Selector: deployment.Spec.Selector,
			Replicas: deployment.Spec.Replicas,
		},
		Status: appsv1.DeploymentStatus{
			Replicas:          deployment.Status.Replicas,
			UpdatedReplicas:   deployment.Status.UpdatedReplicas,
			AvailableReplicas: deployment.Status.AvailableReplicas,
		},
	}
}

// reportFullListDeployment report all deployment list when starts deployment reporter.
func (dr *DeploymentReporter) reportFullListDeployment(ctx *ReporterContext) {
	if ok := cache.WaitForCacheSync(ctx.StopChan, ctx.InformerFactory.Apps().V1().Deployments().Informer().HasSynced); !ok {
		klog.Errorf("failed to wait for caches to sync")
		return
	}

	deploymentList := ctx.InformerFactory.Apps().V1().Deployments().Informer().GetIndexer().ListKeys()

	deploymentMap := &DeploymentResourceStatus{
		FullList: deploymentList,
	}

	go dr.sendToSyncChan(deploymentMap)
}
