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
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/clustermessage"
)

const (
	tickerDuration = 5 * time.Second
)

// PodReporter is responsible for synchronizing pod status of edge cluster.
type PodReporter struct {
	SyncChan chan clustermessage.ClusterMessage

	updatedPodsRWMutex *sync.RWMutex
	updatedPodsMap     *PodResourceStatus

	ctx *ReporterContext
}

// newPodReporter creates a new PodReporter.
func newPodReporter(ctx *ReporterContext) (*PodReporter, error) {
	if !ctx.IsValid() {
		return nil, fmt.Errorf("ReporterContext validation failed")
	}

	podReporter := &PodReporter{
		ctx: ctx,
		updatedPodsMap: &PodResourceStatus{
			UpdateMap: make(map[string]*corev1.Pod),
			DelMap:    make(map[string]*corev1.Pod),
		},
		updatedPodsRWMutex: &sync.RWMutex{},
		SyncChan:           ctx.SyncChan,
	}

	ctx.InformerFactory.Core().V1().Pods().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: podReporter.handlePod,
		UpdateFunc: func(old, new interface{}) {
			newPod := new.(*corev1.Pod)
			oldPod := old.(*corev1.Pod)
			if newPod.ResourceVersion == oldPod.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				return
			}
			podReporter.handlePod(new)
		},
		DeleteFunc: podReporter.deletePod,
	})

	return podReporter, nil
}

// Run begins watching and syncing.
func (pr *PodReporter) Run(stopCh <-chan struct{}) error {
	klog.Info("starting podReporter")

	go wait.Until(pr.sendClusterMessageToSyncChan, tickerDuration, stopCh)

	<-stopCh
	klog.Info("shutting down podReporter")

	return nil
}

// sendClusterMessageToSyncChan sends wrapped ClusterMessage data to SyncChan.
func (pr *PodReporter) sendClusterMessageToSyncChan() {
	pr.updatedPodsRWMutex.Lock()
	defer pr.updatedPodsRWMutex.Unlock()

	// check map length, empty UpdateMap and DelMap don't need to send pod reports
	if len(pr.updatedPodsMap.UpdateMap) == 0 && len(pr.updatedPodsMap.DelMap) == 0 {
		return
	}

	updatedPodsMapJSON, err := json.Marshal(*pr.updatedPodsMap)
	if err != nil {
		klog.Errorf("serialize map failed: %v", err)
		return
	}
	// Define pod report content and convert to json
	data := []Report{
		{
			ResourceType: ResourceTypePod,
			Body:         updatedPodsMapJSON,
		},
	}

	jsonMap, err := json.Marshal(data)
	if err != nil {
		klog.Errorf("serialize report slice failed: %v", err)
		return
	}

	// define pb msg
	msg := &clustermessage.ClusterMessage{
		Head: &clustermessage.MessageHead{
			MessageID:         "",
			Command:           clustermessage.CommandType_EdgeReport,
			ClusterSelector:   "",
			ClusterName:       pr.ctx.ClusterName(),
			ParentClusterName: "",
		},
		Body: jsonMap,
	}

	// send msg to chan
	pr.SyncChan <- *msg

	// clean up the map
	pr.updatedPodsMap.DelMap = make(map[string]*corev1.Pod)
	pr.updatedPodsMap.UpdateMap = make(map[string]*corev1.Pod)
}

// SetUpdateMap adds pod objects to UpdateMap.
func (pr *PodReporter) SetUpdateMap(name string, pod *corev1.Pod) {
	pr.updatedPodsRWMutex.Lock()
	defer pr.updatedPodsRWMutex.Unlock()

	pr.updatedPodsMap.UpdateMap[name] = pod
}

// SetDelMap adds pod objects to DelMap.
func (pr *PodReporter) SetDelMap(name string, pod *corev1.Pod) {
	pr.updatedPodsRWMutex.Lock()
	defer pr.updatedPodsRWMutex.Unlock()

	if _, ok := pr.updatedPodsMap.UpdateMap[name]; ok {
		delete(pr.updatedPodsMap.UpdateMap, name)
	}
	pr.updatedPodsMap.DelMap[name] = pod
}

func startPodReporter(ctx *ReporterContext) error {
	podReporter, err := newPodReporter(ctx)
	if err != nil {
		klog.Errorf("Failed to start pod reporter: %v", err)
		return err
	}

	go podReporter.Run(ctx.StopChan)

	return nil
}

// handlePod is used to handle the creation and update operations of the pod.
func (pr *PodReporter) handlePod(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		klog.Errorf("Should be Pod object but encounter others in handlePod")

		return
	}

	pr.resetPodSpecParameter(&pod.Spec)

	addLabelToResource(&pod.ObjectMeta, pr.ctx)

	key, err := cache.MetaNamespaceKeyFunc(pod)
	if err != nil {
		klog.Errorf("Failed to get map key: %s", err)
		return
	}
	klog.V(3).Infof("find pod : %s", key)

	pr.SetUpdateMap(key, pod)
}

func (pr *PodReporter) resetPodSpecParameter(spec *corev1.PodSpec) {
	for index := range spec.Containers {
		spec.Containers[index].VolumeMounts = nil
	}

	spec.ServiceAccountName = ""
}

// deletePod is used to handle the removal of the pod.
func (pr *PodReporter) deletePod(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		klog.Errorf("Should be Pod object but encounter others in deletePod")

		return
	}

	addLabelToResource(&pod.ObjectMeta, pr.ctx)

	key, err := cache.MetaNamespaceKeyFunc(pod)
	if err != nil {
		klog.Errorf("Failed to get map key: %v", err)
		return
	}

	pr.SetDelMap(key, pod)
}
