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

func newEvent() *corev1.Event {
	return &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		InvolvedObject: corev1.ObjectReference{
			Kind: "Pod",
		},
	}
}

func (f *fixture) newEventReporter() *EventReporter {
	ctx := f.newReportContext()

	eventReporter := &EventReporter{
		ctx:      ctx,
		SyncChan: ctx.SyncChan,
	}

	return eventReporter
}

func TestStartEventReporter(t *testing.T) {
	f := newFixture(t)
	ctx := f.newReportContext()
	err := startEventReporter(ctx)
	assert.Nil(t, err)

	ctx = &ReporterContext{}
	err = startEventReporter(ctx)
	assert.NotNil(t, err)
}

func TestHandleEvent(t *testing.T) {
	f := newFixture(t)
	event := newEvent()
	eventReporter := f.newEventReporter()
	eventReporter.handleEvent(event)

	data := <-eventReporter.SyncChan
	assert.NotNil(t, data)
	assert.Equal(t, clustermessage.CommandType_EdgeReport, data.Head.Command)
	assert.Equal(t, clusterName, data.Head.ClusterName)

	close(eventReporter.SyncChan)
}

func TestDeleteEvent(t *testing.T) {
	f := newFixture(t)
	event := newEvent()
	eventReporter := f.newEventReporter()
	eventReporter.deleteEvent(event)

	data := <-eventReporter.SyncChan
	assert.NotNil(t, data)
	assert.Equal(t, clustermessage.CommandType_EdgeReport, data.Head.Command)
	assert.Equal(t, clusterName, data.Head.ClusterName)

	close(eventReporter.SyncChan)
}

func TestEvSendToSyncChan(t *testing.T) {
	f := newFixture(t)
	event := newEvent()
	eventReporter := f.newEventReporter()

	ers := &EventResourceStatus{
		UpdateMap: map[string]*corev1.Event{
			name: event,
		},
	}

	eventReporter.sendToSyncChan(ers)
	data := <-eventReporter.SyncChan
	assert.NotNil(t, data)
	assert.Equal(t, clustermessage.CommandType_EdgeReport, data.Head.Command)
	assert.Equal(t, clusterName, data.Head.ClusterName)

	close(eventReporter.SyncChan)
}
