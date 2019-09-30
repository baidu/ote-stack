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

package namespace

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/baidu/ote-stack/pkg/clustermessage"
	"github.com/baidu/ote-stack/pkg/controllermanager"
)

var (
	noResyncPeriodFunc = func() time.Duration { return 0 }
)

type fixture struct {
	t           *testing.T
	kubeClient  *k8sfake.Clientset
	kubeObjects []runtime.Object
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{}
	f.t = t
	f.kubeObjects = []runtime.Object{}
	return f
}

func newFakeNamespace(name string) *v1.Namespace {
	return &v1.Namespace{}
}

func newFakeNamespaceController() *NamespaceController {
	namespaceController := &NamespaceController{
		sendChan: make(chan clustermessage.ClusterMessage, 1),
	}

	return namespaceController
}

func TestInitNamespaceController(t *testing.T) {
	f := newFixture(t)
	f.kubeClient = k8sfake.NewSimpleClientset(f.kubeObjects...)

	k8sInformer := informers.NewSharedInformerFactory(f.kubeClient, noResyncPeriodFunc())

	ctx := &controllermanager.ControllerContext{
		K8sContext: controllermanager.K8sContext{
			InformerFactory: k8sInformer,
		},
	}

	err := InitNamespaceController(ctx)
	assert.Nil(t, err)
}

func TestSendNamespaceToCluster(t *testing.T) {
	fakeController := newFakeNamespaceController()
	namespace := newFakeNamespace("")

	err := fakeController.sendNamespaceToCluster(namespace)
	assert.Nil(t, err)
}
