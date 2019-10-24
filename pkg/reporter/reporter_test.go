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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/baidu/ote-stack/pkg/clustermessage"
)

func TestIsValid(t *testing.T) {
	var kubeclient *k8sfake.Clientset
	ctx := &ReporterContext{
		InformerFactory: kubeinformers.NewSharedInformerFactory(kubeclient, func() time.Duration { return 0 }()),
		ClusterName: func() string {
			return "name1"
		},
		SyncChan: make(chan clustermessage.ClusterMessage),
		StopChan: make(<-chan struct{}),
	}

	//ctx.InformerFactory is empty
	ctx.InformerFactory = nil
	ok := ctx.IsValid()
	assert.False(t, ok)

	//ctx.SyncChan is empty
	ctx.SyncChan = nil
	ok = ctx.IsValid()
	assert.False(t, ok)

	//ctx.StopChan is empty
	ctx.StopChan = nil
	ok = ctx.IsValid()
	assert.False(t, ok)

	//ctx is empty
	ctx = nil
	ok = ctx.IsValid()
	assert.False(t, ok)
}

func TestNewReporterInitializers(t *testing.T) {
	reporter := NewReporterInitializers()
	assert.IsType(t, reporter, map[string]InitFunc{})
}

func TestAddLabelToResource(t *testing.T) {
	ctx := &ReporterContext{
		ClusterName: func() string {
			return "c1"
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "1234",
		},
	}

	addLabelToResource(&pod.ObjectMeta, ctx)

	assert.NotNil(t, pod.Labels)
	assert.Equal(t, "c1", pod.Labels[ClusterLabel])
	assert.Equal(t, "1234", pod.Labels[EdgeVersionLabel])
}
