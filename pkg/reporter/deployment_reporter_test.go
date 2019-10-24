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

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/baidu/ote-stack/pkg/clustermessage"
)

func newDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func (f *fixture) newReportContext() *ReporterContext {
	f.kubeclient = k8sfake.NewSimpleClientset(f.kubeobjects...)
	k8sInformer := kubeinformers.NewSharedInformerFactory(f.kubeclient, noResyncPeriodFunc())

	ctx := &ReporterContext{
		InformerFactory: k8sInformer,
		ClusterName: func() string {
			return clusterName
		},
		SyncChan: make(chan clustermessage.ClusterMessage, 1),
		StopChan: make(<-chan struct{}),
	}

	return ctx
}

func (f *fixture) newDeploymentReporter() *DeploymentReporter {
	ctx := f.newReportContext()

	deploymentReporter := &DeploymentReporter{
		ctx:      ctx,
		SyncChan: ctx.SyncChan,
	}

	return deploymentReporter
}

func TestStartDeploymentReporter(t *testing.T) {
	f := newFixture(t)
	ctx := f.newReportContext()
	err := startDeploymentReporter(ctx)
	assert.Nil(t, err)

	ctx = &ReporterContext{}
	err = startDeploymentReporter(ctx)
	assert.NotNil(t, err)
}

func TestHandleDeployment(t *testing.T) {
	f := newFixture(t)
	deployment := newDeployment()
	deploymentReporter := f.newDeploymentReporter()
	deploymentReporter.handleDeployment(deployment)

	data := <-deploymentReporter.SyncChan
	assert.NotNil(t, data)
	assert.Equal(t, clustermessage.CommandType_EdgeReport, data.Head.Command)
	assert.Equal(t, clusterName, data.Head.ClusterName)

	close(deploymentReporter.SyncChan)
}

func TestDeleteDeployment(t *testing.T) {
	f := newFixture(t)
	deployment := newDeployment()
	deploymentReporter := f.newDeploymentReporter()
	deploymentReporter.deleteDeployment(deployment)

	data := <-deploymentReporter.SyncChan
	assert.NotNil(t, data)
	assert.Equal(t, clustermessage.CommandType_EdgeReport, data.Head.Command)
	assert.Equal(t, clusterName, data.Head.ClusterName)

	close(deploymentReporter.SyncChan)
}

func TestDpSendToSyncChan(t *testing.T) {
	f := newFixture(t)
	deployment := newDeployment()
	deploymentReporter := f.newDeploymentReporter()

	drs := &DeploymentResourceStatus{
		UpdateMap: map[string]*appsv1.Deployment{
			name: deployment,
		},
	}

	deploymentReporter.sendToSyncChan(drs)
	data := <-deploymentReporter.SyncChan
	assert.NotNil(t, data)
	assert.Equal(t, clustermessage.CommandType_EdgeReport, data.Head.Command)
	assert.Equal(t, clusterName, data.Head.ClusterName)

	close(deploymentReporter.SyncChan)
}
