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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/baidu/ote-stack/pkg/clustermessage"
)

var (
	noResyncPeriodFunc = func() time.Duration { return 0 }
	clusterName        = "test123"
	name               = "test-pod-name"
	namespace          = "test-pod-namespace"
	mapKey             = "test-pod-namespace/test-pod-name"
)

type fixture struct {
	t *testing.T

	kubeclient *k8sfake.Clientset

	// Objects from here preloaded into NewSimpleFake
	kubeobjects []runtime.Object
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{}
	f.t = t
	f.kubeobjects = []runtime.Object{}
	return f
}

func (f *fixture) newPodReporter() *PodReporter {
	f.kubeclient = k8sfake.NewSimpleClientset(f.kubeobjects...)
	k8sI := kubeinformers.NewSharedInformerFactory(f.kubeclient, noResyncPeriodFunc())

	ctx := &ReporterContext{
		InformerFactory: k8sI,
		ClusterName: func() string {
			return clusterName
		},
		SyncChan: make(chan clustermessage.ClusterMessage),
		StopChan: make(<-chan struct{}),
	}

	podReporter, err := newPodReporter(ctx)
	assert.Nil(f.t, err)

	return podReporter
}

func TestNewPodReporter(t *testing.T) {
	f := newFixture(t)
	podReporter := f.newPodReporter()
	assert.Equal(t, clusterName, podReporter.ctx.ClusterName())
}

func TestHandlePod(t *testing.T) {
	f := newFixture(t)
	podReporter := f.newPodReporter()

	pod := newPod()
	podReporter.handlePod(pod)
	for key, pod := range podReporter.updatedPodsMap.UpdateMap {
		if key == mapKey {
			assert.Equal(t, namespace, pod.Namespace)
			assert.Equal(t, name, pod.Name)
		}
	}
	// not pod Object
	podReporter.handlePod(struct{}{})
}

func TestDeletePod(t *testing.T) {
	f := newFixture(t)
	podReporter := f.newPodReporter()

	pod := newPod()
	podReporter.deletePod(pod)
	for key, pod := range podReporter.updatedPodsMap.UpdateMap {
		if key == mapKey {
			assert.Equal(t, namespace, pod.Namespace)
			assert.Equal(t, name, pod.Name)
		}
	}
	// not pod Object
	podReporter.deletePod(struct{}{})
}

func TestRun(t *testing.T) {
	f := newFixture(t)
	podReporter := f.newPodReporter()

	// delete pod
	pod := newPod()
	podReporter.deletePod(pod)

	// update/create pod
	pod.Name = "update123"
	pod.Namespace = "update456"
	podReporter.handlePod(pod)

	stopChan := make(chan struct{})

	go podReporter.Run(stopChan)

	data := <-podReporter.SyncChan
	if data.Head.Command == clustermessage.CommandType_EdgeReport {
		assert.Equal(t, clusterName, data.Head.ClusterName)

		ret := []Report{}
		err := json.Unmarshal(data.Body, &ret)
		assert.Nil(t, err)

		tmp := PodResourceStatus{}
		err = json.Unmarshal(ret[0].Body, &tmp)
		assert.Nil(t, err)

		pod := tmp.UpdateMap["update456/update123"]
		assert.IsType(t, &corev1.Pod{}, pod)
		assert.Equal(t, "update123", pod.Name)
		assert.Equal(t, "update456", pod.Namespace)
	}
	// test clean map
	assert.Empty(t, podReporter.updatedPodsMap.UpdateMap)

	stopChan <- struct{}{}
	close(podReporter.SyncChan)
}

func newPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{ClusterLabel: clusterName},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
}
