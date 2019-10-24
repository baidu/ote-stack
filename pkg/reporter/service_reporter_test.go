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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/baidu/ote-stack/pkg/clustermessage"
)

func newService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func (f *fixture) newServiceReporter() *ServiceReporter {
	ctx := f.newReportContext()

	serviceReporter := &ServiceReporter{
		ctx:      ctx,
		SyncChan: ctx.SyncChan,
	}

	return serviceReporter
}

func TestStartServiceReporter(t *testing.T) {
	f := newFixture(t)
	ctx := f.newReportContext()
	err := startServiceReporter(ctx)
	assert.Nil(t, err)

	ctx = &ReporterContext{}
	err = startServiceReporter(ctx)
	assert.NotNil(t, err)
}

func TestHandleService(t *testing.T) {
	f := newFixture(t)
	service := newService()
	serviceReporter := f.newServiceReporter()
	serviceReporter.handleService(service)

	data := <-serviceReporter.SyncChan
	assert.NotNil(t, data)
	assert.Equal(t, clustermessage.CommandType_EdgeReport, data.Head.Command)
	assert.Equal(t, clusterName, data.Head.ClusterName)

	close(serviceReporter.SyncChan)
}

func TestDeleteService(t *testing.T) {
	f := newFixture(t)
	service := newService()
	serviceReporter := f.newServiceReporter()
	serviceReporter.deleteService(service)

	data := <-serviceReporter.SyncChan
	assert.NotNil(t, data)
	assert.Equal(t, clustermessage.CommandType_EdgeReport, data.Head.Command)
	assert.Equal(t, clusterName, data.Head.ClusterName)

	close(serviceReporter.SyncChan)
}

func TestSvcSendToSyncChan(t *testing.T) {
	f := newFixture(t)
	service := newService()
	serviceReporter := f.newServiceReporter()

	srs := &ServiceResourceStatus{
		UpdateMap: map[string]*corev1.Service{
			name: service,
		},
	}

	serviceReporter.sendToSyncChan(srs)
	data := <-serviceReporter.SyncChan
	assert.NotNil(t, data)
	assert.Equal(t, clustermessage.CommandType_EdgeReport, data.Head.Command)
	assert.Equal(t, clusterName, data.Head.ClusterName)

	close(serviceReporter.SyncChan)
}
