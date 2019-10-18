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
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	kubernetes "k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"

	"github.com/baidu/ote-stack/pkg/clustermessage"
	"github.com/baidu/ote-stack/pkg/reporter"
)

var (
	scheme             = runtime.NewScheme()
	codecs             = serializer.NewCodecFactory(scheme)
	localSchemeBuilder = runtime.SchemeBuilder{
		corev1.AddToScheme,
	}
	addToScheme = localSchemeBuilder.AddToScheme
)

func TestHandleReceivedMessage(t *testing.T) {
	u := NewUpstreamProcessor(&K8sContext{})
	// get msg failed
	err := u.HandleReceivedMessage("", nil)
	assert.NotNil(t, err)

	// get msg with nil Head
	msg := &clustermessage.ClusterMessage{}
	data, err := msg.Serialize()
	assert.NotNil(t, data)
	assert.Nil(t, err)
	err = u.HandleReceivedMessage("", data)
	assert.NotNil(t, err)

	// get msg with command not supported(Reserved)
	msg.Head = &clustermessage.MessageHead{Command: clustermessage.CommandType_Reserved}
	data, err = msg.Serialize()
	assert.NotNil(t, data)
	assert.Nil(t, err)
	err = u.HandleReceivedMessage("", data)
	assert.NotNil(t, err)

	// get msg with command EdgeReport
	// TODO detail assert
	podUpdatesMap := &reporter.PodResourceStatus{
		UpdateMap: make(map[string]*corev1.Pod),
		DelMap:    make(map[string]*corev1.Pod),
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-name1",
			Namespace:       "test-namespace1",
			Labels:          map[string]string{reporter.ClusterLabel: "cluster1"},
			ResourceVersion: "10",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
	podUpdatesMap.UpdateMap["test-namespace1/test-name1"] = pod
	podUpdatesMap.DelMap["test-namespace1/test-name2"] = pod

	podUpdatesMapJSON, err := json.Marshal(*podUpdatesMap)
	assert.Nil(t, err)

	reportData := []reporter.Report{
		{
			ResourceType: reporter.ResourceTypePod,
			Body:         podUpdatesMapJSON,
		},
	}

	body, err := json.Marshal(reportData)
	assert.Nil(t, err)

	msg.Head.Command = clustermessage.CommandType_EdgeReport
	msg.Body = body

	data, err = msg.Serialize()
	assert.NotNil(t, data)
	assert.Nil(t, err)

	mockClient := &kubernetes.Clientset{}
	mockClient.AddReactor("get", "pods", func(action kubetesting.Action) (bool, runtime.Object, error) {
		return true, nil, kubeerrors.NewNotFound(schema.GroupResource{}, "")
	})
	u.ctx.K8sClient = mockClient
	err = u.HandleReceivedMessage("", data)
	assert.Nil(t, err)
}

func init() {
	metav1.AddToGroupVersion(scheme, schema.GroupVersion{Version: "v1"})
	utilruntime.Must(addToScheme(scheme))
}

// client-go v10 version not found Tracker()
func newSimpleClientset(objects ...runtime.Object) (*kubernetes.Clientset, kubetesting.ObjectTracker) {
	o := kubetesting.NewObjectTracker(scheme, codecs.UniversalDecoder())
	for _, obj := range objects {
		if err := o.Add(obj); err != nil {
			panic(err)
		}
	}

	cs := &kubernetes.Clientset{}
	//cs.discovery = &fakediscovery.FakeDiscovery{Fake: &cs.Fake}
	cs.AddReactor("*", "*", kubetesting.ObjectReaction(o))
	cs.AddWatchReactor("*", func(action kubetesting.Action) (handled bool, ret watch.Interface, err error) {
		gvr := action.GetResource()
		ns := action.GetNamespace()
		watch, err := o.Watch(gvr, ns)
		if err != nil {
			return false, nil, err
		}
		return true, watch, nil
	})

	return cs, o
}
