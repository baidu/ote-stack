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

package clustercrd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	otev1 "github.com/baidu/ote-stack/pkg/apis/ote/v1"
	"github.com/baidu/ote-stack/pkg/clustermessage"
	"github.com/baidu/ote-stack/pkg/controllermanager"
	"github.com/baidu/ote-stack/pkg/generated/clientset/versioned/fake"
	oteinformer "github.com/baidu/ote-stack/pkg/generated/informers/externalversions"
)

var (
	noResyncPeriodFunc = func() time.Duration { return 0 }
)

type fixture struct {
	t           *testing.T
	client      *fake.Clientset
	kubeClient  *k8sfake.Clientset
	objects     []runtime.Object
	kubeObjects []runtime.Object
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{}
	f.t = t
	f.objects = []runtime.Object{}
	f.kubeObjects = []runtime.Object{}
	return f
}

func newFakeCluster(name string) *otev1.Cluster {
	return &otev1.Cluster{
		Spec: otev1.ClusterSpec{
			Name: name,
		},
	}
}

func (f *fixture) newFakeClusterCrdController() *ClusterCrdController {
	f.kubeClient = k8sfake.NewSimpleClientset(f.kubeObjects...)

	clusterCrdController := &ClusterCrdController{
		sendChan:  make(chan clustermessage.ClusterMessage, 1),
		k8sClient: f.kubeClient,
	}
	return clusterCrdController
}

func TestInitClusterCrdController(t *testing.T) {
	f := newFixture(t)
	f.client = fake.NewSimpleClientset(f.objects...)
	f.kubeClient = k8sfake.NewSimpleClientset(f.kubeObjects...)

	k8sInformer := oteinformer.NewSharedInformerFactory(f.client, noResyncPeriodFunc())

	ctx := &controllermanager.ControllerContext{
		K8sContext: controllermanager.K8sContext{
			OteInformerFactory: k8sInformer,
			K8sClient:          f.kubeClient,
		},
	}

	err := InitClusterCrdController(ctx)
	assert.Nil(t, err)
}

func TestSendNamespaceToNewCluster(t *testing.T) {
	f := newFixture(t)
	fakeController := f.newFakeClusterCrdController()

	cluster := newFakeCluster("")
	err := fakeController.sendNamespaceToNewCluster(cluster)
	assert.Nil(t, err)
}

func TestGetNamespaceList(t *testing.T) {
	f := newFixture(t)
	fakeController := f.newFakeClusterCrdController()
	ret, err := fakeController.getNamespaceList()
	assert.Nil(t, err)
	assert.Nil(t, ret)
}
