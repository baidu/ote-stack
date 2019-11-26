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

func (u *UpstreamProcessor) handlePodReport(b []byte) error {
	// Deserialize byte data to PodReportStatus
	prs, err := PodReportStatusDeserialize(b)
	if err != nil {
		return fmt.Errorf("PodReportStatusDeserialize failed : %v", err)
	}
	// handle FullList
	if prs.FullList != nil {
		// TODO:handle full pod resource.
	}
	// handle UpdateMap
	if prs.UpdateMap != nil {
		u.handlePodUpdateMap(prs.UpdateMap)
	}
	// handle DelMap
	if prs.DelMap != nil {
		u.handlePodDelMap(prs.DelMap)
	}

	return nil
}

func (u *UpstreamProcessor) handlePodDelMap(delMap map[string]*corev1.Pod) {
	for _, pod := range delMap {
		err := UniqueResourceName(&pod.ObjectMeta)
		if err != nil {
			klog.Errorf("handlePodDelMap's UniqueResourceName method failed, %s", err)
			continue
		}

		err = u.DeletePod(pod)
		if err != nil {
			klog.Errorf("Report pod delete event failed : %v", err)
			continue
		}

		klog.V(3).Infof("Report pod delete event success: namespace(%s), name(%s)", pod.Namespace, pod.Name)
	}
}

func (u *UpstreamProcessor) handlePodUpdateMap(updateMap map[string]*corev1.Pod) {
	for _, pod := range updateMap {
		err := UniqueResourceName(&pod.ObjectMeta)
		if err != nil {
			klog.Errorf("handlePodUpdateMap's UniqueResourceName method failed, %s", err)
			continue
		}

		err = u.CreateOrUpdatePod(pod)
		if err != nil {
			klog.Errorf("Report pod create or update event failed : %v", err)
			continue
		}
	}
}

//PodReportStatusDeserialize deserialize byte data to PodReportStatus.
func PodReportStatusDeserialize(b []byte) (*reporter.PodResourceStatus, error) {
	podReportStatus := reporter.PodResourceStatus{}
	err := json.Unmarshal(b, &podReportStatus)
	if err != nil {
		return nil, err
	}
	return &podReportStatus, nil
}

// GetPod will retrieve the requested pod based on namespace and name.
func (u *UpstreamProcessor) GetPod(pod *corev1.Pod) (*corev1.Pod, error) {
	storedPod, err := u.ctx.K8sClient.CoreV1().Pods(pod.Namespace).Get(pod.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return storedPod, err
}

// CreatePod will create the given pod.
func (u *UpstreamProcessor) CreatePod(pod *corev1.Pod) error {
	// ResourceVersion should not be assigned at creation time
	pod.ResourceVersion = ""
	_, err := u.ctx.K8sClient.CoreV1().Pods(pod.Namespace).Create(pod)

	if err != nil {
		return err
	}
	klog.V(3).Infof("Report pod create event success: namespace(%s), name(%s)", pod.Namespace, pod.Name)

	return nil
}

// UpdatePod will update the given pod.
func (u *UpstreamProcessor) UpdatePod(pod *corev1.Pod) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		storedPod, err := u.GetPod(pod)
		if err != nil {
			return err
		}

		if !checkEdgeVersion(&pod.ObjectMeta, &storedPod.ObjectMeta) {
			return fmt.Errorf("check pod edge version failed")
		}

		adaptToCentralResource(&pod.ObjectMeta, &storedPod.ObjectMeta)

		pod.Spec.NodeName = storedPod.Spec.NodeName

		_, err = u.ctx.K8sClient.CoreV1().Pods(storedPod.Namespace).Update(pod)
		return err
	})

	if err != nil {
		return err

	}

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		storedPod, err := u.GetPod(pod)
		if err != nil {
			return err
		}

		if !checkEdgeVersion(&pod.ObjectMeta, &storedPod.ObjectMeta) {
			return fmt.Errorf("check pod edge version failed")
		}

		adaptToCentralResource(&pod.ObjectMeta, &storedPod.ObjectMeta)

		_, err = u.ctx.K8sClient.CoreV1().Pods(storedPod.Namespace).UpdateStatus(pod)
		return err
	})

	if err != nil {
		return err
	}

	klog.V(3).Infof("Report pod update event success: namespace(%s), name(%s)", pod.Namespace, pod.Name)

	return nil
}

// CreateOrUpdatePod will update the given pod or create it if does not exist.
func (u *UpstreamProcessor) CreateOrUpdatePod(pod *corev1.Pod) error {
	// The Pod is not "delete state". At this time, the status of the Pod is still running.
	// The so-called "delete state" is only the deleteTimestamp and deleteGracePeriodSeconds fields are set.
	// So if the deleteTimestamp field exists, delete it.
	if pod.DeletionTimestamp != nil {
		return u.DeletePod(pod)
	}

	_, err := u.GetPod(pod)
	// If not found resource, create it.
	if err != nil && errors.IsNotFound(err) {
		err = u.CreatePod(pod)
	}

	if err != nil {
		return err
	}

	return u.UpdatePod(pod)
}

// DeletePod will delete the given pod.
func (u *UpstreamProcessor) DeletePod(pod *corev1.Pod) error {
	return u.ctx.K8sClient.CoreV1().Pods(pod.Namespace).Delete(pod.Name, &metav1.DeleteOptions{
		GracePeriodSeconds: &noGracePeriodSeconds,
	})
}
