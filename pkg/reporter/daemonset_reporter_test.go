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

	"github.com/baidu/ote-stack/pkg/clustermessage"
)

func newDaemonset() *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func (f *fixture) newDaemonsetReporter() *DaemonsetReporter {
	ctx := f.newReportContext()

	daemonsetReporter := &DaemonsetReporter{
		ctx:      ctx,
		SyncChan: ctx.SyncChan,
	}

	return daemonsetReporter
}

func TestStartDaemonsetReporter(t *testing.T) {
	f := newFixture(t)
	ctx := f.newReportContext()
	err := startDaemonsetReporter(ctx)
	assert.Nil(t, err)

	ctx = &ReporterContext{}
	err = startDaemonsetReporter(ctx)
	assert.NotNil(t, err)
}

func TestHandleDaemonset(t *testing.T) {
	f := newFixture(t)
	daemonset := newDaemonset()
	daemonsetReporter := f.newDaemonsetReporter()
	daemonsetReporter.handleDaemonset(daemonset)

	data := <-daemonsetReporter.SyncChan
	assert.NotNil(t, data)
	assert.Equal(t, clustermessage.CommandType_EdgeReport, data.Head.Command)
	assert.Equal(t, clusterName, data.Head.ClusterName)

	close(daemonsetReporter.SyncChan)
}

func TestDeleteDaemonset(t *testing.T) {
	f := newFixture(t)
	daemonset := newDaemonset()
	daemonsetReporter := f.newDaemonsetReporter()
	daemonsetReporter.deleteDaemonset(daemonset)

	data := <-daemonsetReporter.SyncChan
	assert.NotNil(t, data)
	assert.Equal(t, clustermessage.CommandType_EdgeReport, data.Head.Command)
	assert.Equal(t, clusterName, data.Head.ClusterName)

	close(daemonsetReporter.SyncChan)
}

func TestDsSendToSyncChan(t *testing.T) {
	f := newFixture(t)
	daemonset := newDaemonset()
	daemonsetReporter := f.newDaemonsetReporter()

	drs := &DaemonsetResourceStatus{
		UpdateMap: map[string]*appsv1.DaemonSet{
			name: daemonset,
		},
	}

	daemonsetReporter.sendToSyncChan(drs)
	data := <-daemonsetReporter.SyncChan
	assert.NotNil(t, data)
	assert.Equal(t, clustermessage.CommandType_EdgeReport, data.Head.Command)
	assert.Equal(t, clusterName, data.Head.ClusterName)

	close(daemonsetReporter.SyncChan)
}
