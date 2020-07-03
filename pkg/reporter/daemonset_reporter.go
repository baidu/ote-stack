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

type DaemonsetReporter struct {
	SyncChan chan clustermessage.ClusterMessage
	ctx      *ReporterContext
}

// startDaemonsetReporter inits daemonset reporter and starts to watch daemonset resource.
func startDaemonsetReporter(ctx *ReporterContext) error {
	if !ctx.IsValid() {
		return fmt.Errorf("ReporterContext validation failed")
	}

	daemonsetReporter := &DaemonsetReporter{
		ctx:      ctx,
		SyncChan: ctx.SyncChan,
	}

	// Regists EventHandler for daemonset informer listing and watching daemonset resource.
	// Although daemonset has another API version, extensions/v1beta1，the apps/v1 version is the official stable version.
	// Just use the apps/v1 version here.
	ctx.InformerFactory.Apps().V1().DaemonSets().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: daemonsetReporter.handleDaemonset,
		UpdateFunc: func(old, new interface{}) {
			newDaemonset := new.(*appsv1.DaemonSet)
			oldDaemonset := old.(*appsv1.DaemonSet)
			if newDaemonset.ResourceVersion == oldDaemonset.ResourceVersion {
				// Periodic resync will send update events for all known Daemonset.
				// Two different versions of the same Daemonset will always have different RVs.
				return
			}
			daemonsetReporter.handleDaemonset(new)
		},
		DeleteFunc: daemonsetReporter.deleteDaemonset,
	})

	go daemonsetReporter.reportFullListDaemonset(ctx)

	return nil
}

// handleDaemonset is used to handle the creation and update operations of the daemonset.
func (dr *DaemonsetReporter) handleDaemonset(obj interface{}) {
	daemonset, ok := obj.(*appsv1.DaemonSet)
	if !ok {
		klog.Errorf("Should be Daemonset object but encounter others in handleDaemonset.")
		return
	}
	klog.V(3).Infof("handle Daemonset: %s", daemonset.Name)

	addLabelToResource(&daemonset.ObjectMeta, dr.ctx)

	if dr.ctx.IsLightweightReport {
		daemonset = dr.lightWeightDaemonset(daemonset)
	}

	// generates unique key for daemonset.
	key, err := cache.MetaNamespaceKeyFunc(daemonset)
	if err != nil {
		klog.Errorf("Failed to get map key: %s", err)
		return
	}

	daemonsetMap := &DaemonsetResourceStatus{
		UpdateMap: map[string]*appsv1.DaemonSet{
			key: daemonset,
		},
	}

	go dr.sendToSyncChan(daemonsetMap)
}

// deleteDaemonset is used to handle the removal of the daemonset.
func (dr *DaemonsetReporter) deleteDaemonset(obj interface{}) {
	daemonset, ok := obj.(*appsv1.DaemonSet)
	if !ok {
		klog.Errorf("Should be Daemonset object but encounter others in deleteDaemonset")
		return
	}
	klog.V(3).Infof("Daemonset: %s deleted.", daemonset.Name)

	addLabelToResource(&daemonset.ObjectMeta, dr.ctx)

	if dr.ctx.IsLightweightReport {
		daemonset = dr.lightWeightDaemonset(daemonset)
	}

	// generates unique key for daemonset.
	key, err := cache.MetaNamespaceKeyFunc(daemonset)
	if err != nil {
		klog.Errorf("Failed to get map key: %s", err)
		return
	}

	daemonsetMap := &DaemonsetResourceStatus{
		DelMap: map[string]*appsv1.DaemonSet{
			key: daemonset,
		},
	}

	go dr.sendToSyncChan(daemonsetMap)
}

// sendToSyncChan sends wrapped ClusterMessage data to SyncChan.
func (dr *DaemonsetReporter) sendToSyncChan(daemonsetMap *DaemonsetResourceStatus) {
	daemonsetReports, err := daemonsetMap.serializeMapToReporters()
	if err != nil {
		klog.Errorf("serialize map failed: %v", err)
		return
	}

	msg, err := daemonsetReports.ToClusterMessage(dr.ctx.ClusterName())
	if err != nil {
		klog.Errorf("change daemonset Reports to clustermessage failed: %v", err)
		return
	}

	dr.SyncChan <- *msg
}

// serializeMapToReporters serializes DaemonsetResourceStatus and converts to Reports.
func (ds *DaemonsetResourceStatus) serializeMapToReporters() (Reports, error) {
	daemonsetJson, err := json.Marshal(ds)
	if err != nil {
		return nil, err
	}

	data := Reports{
		{
			ResourceType: ResourceTypeDaemonset,
			Body:         daemonsetJson,
		},
	}

	return data, nil
}

// lightWeightDaemonset crops the content of the daemonset
func (dr *DaemonsetReporter) lightWeightDaemonset(daemonset *appsv1.DaemonSet) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		TypeMeta: daemonset.TypeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name:      daemonset.Name,
			Namespace: daemonset.Namespace,
			Labels:    daemonset.Labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: daemonset.Spec.Selector,
			Template: daemonset.Spec.Template,
		},
		Status: appsv1.DaemonSetStatus{
			CurrentNumberScheduled: daemonset.Status.CurrentNumberScheduled,
			DesiredNumberScheduled: daemonset.Status.DesiredNumberScheduled,
			NumberReady:            daemonset.Status.NumberReady,
			UpdatedNumberScheduled: daemonset.Status.UpdatedNumberScheduled,
			NumberAvailable:        daemonset.Status.NumberAvailable,
		},
	}
}

// reportFullListDaemonset report all daemonset list when starts daemonset reporter.
func (dr *DaemonsetReporter) reportFullListDaemonset(ctx *ReporterContext) {
	if ok := cache.WaitForCacheSync(ctx.StopChan, ctx.InformerFactory.Apps().V1().DaemonSets().Informer().HasSynced); !ok {
		klog.Errorf("failed to wait for caches to sync")
		return
	}

	daemonsetList := ctx.InformerFactory.Apps().V1().DaemonSets().Informer().GetIndexer().ListKeys()

	daemonsetMap := &DaemonsetResourceStatus{
		FullList: daemonsetList,
	}

	go dr.sendToSyncChan(daemonsetMap)
}
