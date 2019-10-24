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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubernetes "k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"

	"github.com/baidu/ote-stack/pkg/reporter"
)

var (
	eventGroup = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "events"}
)

func newEventGetAction(name string) kubetesting.GetActionImpl {
	return kubetesting.NewGetAction(eventGroup, "", name)
}

func newEventCreateAction(event *corev1.Event) kubetesting.CreateActionImpl {
	return kubetesting.NewCreateAction(eventGroup, "", event)
}

func newEventUpdateAction(event *corev1.Event) kubetesting.UpdateActionImpl {
	return kubetesting.NewUpdateAction(eventGroup, "", event)
}

func newEventDeleteAction(name string) kubetesting.DeleteActionImpl {
	return kubetesting.NewDeleteAction(eventGroup, "", name)
}

func NewEvent(name string,
	clusterLabel string, edgeVersion string, resourceVersion string) *corev1.Event {
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Labels:          map[string]string{reporter.ClusterLabel: clusterLabel, reporter.EdgeVersionLabel: edgeVersion},
			ResourceVersion: resourceVersion,
		},
	}

	return event
}

func TestHandleEventReport(t *testing.T) {
	u := NewUpstreamProcessor(&K8sContext{})

	eventUpdateMap := &reporter.EventResourceStatus{
		UpdateMap: make(map[string]*corev1.Event),
		DelMap:    make(map[string]*corev1.Event),
	}

	event := NewEvent("test-name1", "cluster1", "", "1")

	eventUpdateMap.UpdateMap["test-namespace1/test-name1"] = event
	eventUpdateMap.DelMap["test-namespace1/test-name2"] = event

	eventJson, err := json.Marshal(eventUpdateMap)
	assert.Nil(t, err)

	eventReport := reporter.Report{
		ResourceType: reporter.ResourceTypeEvent,
		Body:         eventJson,
	}

	reportJson, err := json.Marshal(eventReport)
	assert.Nil(t, err)

	err = u.handleEventReport(reportJson)
	assert.Nil(t, err)

	err = u.handleEventReport([]byte{1})
	assert.NotNil(t, err)
}

func TestGetCreateEvent(t *testing.T) {
	event1 := NewEvent("test1", "", "11", "10")

	tests := []struct {
		name            string
		event           *corev1.Event
		getEventResult  *corev1.Event
		errorOnGet      error
		errorOnCreation error
		errorOnUpdate   error
		expectActions   []kubetesting.Action
		expectErr       bool
	}{
		{
			name:       "Success to create a new event.",
			event:      event1,
			errorOnGet: kubeerrors.NewNotFound(schema.GroupResource{}, ""),
			expectActions: []kubetesting.Action{
				newEventCreateAction(event1),
			},
			expectErr: false,
		},
		{
			name:            "A error occurs when create a new event fails.",
			event:           event1,
			errorOnGet:      kubeerrors.NewNotFound(schema.GroupResource{}, ""),
			errorOnCreation: errors.New("wanted error"),
			expectActions: []kubetesting.Action{
				newEventCreateAction(event1),
			},
			expectErr: true,
		},
	}

	u := NewUpstreamProcessor(&K8sContext{})

	for _, test := range tests {
		t.Run(test.name, func(t1 *testing.T) {
			assert := assert.New(t1)

			mockClient := &kubernetes.Clientset{}

			mockClient.AddReactor("create", "events", func(action kubetesting.Action) (bool, runtime.Object, error) {
				return true, nil, test.errorOnCreation
			})

			u.ctx.K8sClient = mockClient
			err := u.CreateEvent(test.event)

			if test.expectErr {
				assert.Error(err)
			} else {
				assert.NoError(err)
				// Check calls to kubernetes.
				assert.Equal(test.expectActions, mockClient.Actions())
			}
		})
	}
}

func TestDeleteEvent(t *testing.T) {
	testEvent := NewEvent("test1", "", "11", "10")

	tests := []struct {
		name           string
		event          *corev1.Event
		getEventResult *corev1.Event
		errorOnGet     error
		errorOnDelete  error
		expectActions  []kubetesting.Action
		expectErr      bool
	}{
		{
			name:  "Success to delete an existent event.",
			event: testEvent,
			expectActions: []kubetesting.Action{
				newEventDeleteAction(testEvent.Name),
			},
			expectErr: false,
		},
		{
			name:          "A error occurs when delete a event fails.",
			event:         testEvent,
			errorOnDelete: errors.New("wanted error"),
			expectActions: []kubetesting.Action{
				newEventDeleteAction(testEvent.Name),
			},
			expectErr: true,
		},
	}

	u := NewUpstreamProcessor(&K8sContext{})

	for _, test := range tests {
		t.Run(test.name, func(t1 *testing.T) {
			assert := assert.New(t1)

			mockClient := &kubernetes.Clientset{}
			mockClient.AddReactor("delete", "events", func(action kubetesting.Action) (bool, runtime.Object, error) {
				return true, nil, test.errorOnDelete
			})

			u.ctx.K8sClient = mockClient
			err := u.DeleteEvent(test.event)

			if test.expectErr {
				assert.Error(err)
			} else {
				assert.NoError(err)
				// Check calls to kubernetes.
				assert.Equal(test.expectActions, mockClient.Actions())
			}
		})
	}
}

func TestRelateToPodOrNode(t *testing.T) {
	u := NewUpstreamProcessor(&K8sContext{})

	mockClient := &kubernetes.Clientset{}
	mockClient.AddReactor("get", "pods", func(action kubetesting.Action) (bool, runtime.Object, error) {
		storedPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:     "test-c1",
				SelfLink: "/api/v1/namespaces/default/pods/test-c1",
			},
		}

		return true, storedPod, nil
	})

	mockClient.AddReactor("get", "nodes", func(action kubetesting.Action) (bool, runtime.Object, error) {
		storedNode := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:     "test-c2",
				SelfLink: "/api/v1/nodes/test-c2",
			},
		}

		return true, storedNode, nil
	})

	u.ctx.K8sClient = mockClient

	// test pod's event
	event1 := NewEvent("test-event", "c1", "", "")
	event1.InvolvedObject.Name = "test"
	event1.InvolvedObject.Namespace = "test"
	event1.InvolvedObject.Kind = "Pod"

	err := u.relateToPodOrNode(event1)

	assert.Nil(t, err)
	assert.Equal(t, "test-c1", event1.InvolvedObject.Name)

	// test node's event
	event2 := NewEvent("test-event", "c2", "", "")
	event2.InvolvedObject.Name = "test"
	event2.InvolvedObject.Namespace = "test"
	event2.InvolvedObject.Kind = "Node"

	err = u.relateToPodOrNode(event2)
	assert.Nil(t, err)
	assert.Equal(t, "test-c2", event2.InvolvedObject.Name)

	// test other resource type's event
	event3 := NewEvent("test-event", "c3", "", "")
	event3.InvolvedObject.Name = "test"
	event3.InvolvedObject.Namespace = "test"
	event3.InvolvedObject.Kind = "Service"

	err = u.relateToPodOrNode(event3)
	assert.NotNil(t, err)
	assert.NotEqual(t, "test-c3", event3.InvolvedObject.Name)
}

func TestHandleEvent(t *testing.T) {
	event1 := NewEvent("test-c1", "c1", "12", "")
	event1.InvolvedObject.Name = "test"
	event1.InvolvedObject.Namespace = "test"
	event1.InvolvedObject.Kind = "Pod"

	mockClient := &kubernetes.Clientset{}
	mockClient.AddReactor("get", "pods", func(action kubetesting.Action) (bool, runtime.Object, error) {
		storedPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:     "test-c1",
				SelfLink: "/api/v1/namespaces/default/pods/test-c1",
			},
		}

		return true, storedPod, nil
	})

	eventStatus := reporter.EventResourceStatus{
		UpdateMap: map[string]*corev1.Event{
			"test-c1": event1,
		},
		DelMap: map[string]*corev1.Event{
			"test-c1": event1,
		},
	}

	eventJson, err := json.Marshal(eventStatus)
	assert.Nil(t, err)
	assert.NotNil(t, eventJson)

	u := NewUpstreamProcessor(&K8sContext{})
	u.ctx.K8sClient = mockClient

	err = u.handleEventReport(eventJson)
	assert.Nil(t, err)
}
