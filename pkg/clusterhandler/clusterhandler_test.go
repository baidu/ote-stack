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

package clusterhandler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	otev1 "github.com/baidu/ote-stack/pkg/apis/ote/v1"
	clusterrouter "github.com/baidu/ote-stack/pkg/clusterrouter"
	"github.com/baidu/ote-stack/pkg/config"
	oteclient "github.com/baidu/ote-stack/pkg/generated/clientset/versioned/fake"
	"github.com/baidu/ote-stack/pkg/tunnel"
)

var (
	fakeTunn = newFakeCloudTunnel()
)

func TestInit(t *testing.T) {
	c := &config.ClusterControllerConfig{}
	client := oteclient.NewSimpleClientset()

	h := &clusterHandler{
		conf:      c,
		k8sEnable: false,
	}
	h.conf.ClusterUserDefineName = ""
	assert.Error(t, h.valid())

	// test root cluster config
	h.conf.ClusterUserDefineName = config.ROOT_CLUSTER_NAME
	assert.Error(t, h.valid())
	h.conf.TunnelListenAddr = ":8272"
	assert.Error(t, h.valid())
	h.conf.K8sClient = client
	assert.False(t, h.k8sEnable)
	assert.NoError(t, h.valid())
	assert.True(t, h.k8sEnable)
	h.conf.ParentCluster = "parent"
	assert.Error(t, h.valid())

	// test no-root cluster config
	h.conf.ClusterUserDefineName = "c1"
	h.conf.ParentCluster = ""
	assert.Error(t, h.valid())
	h.conf.ParentCluster = "parent"
	assert.NoError(t, h.valid())

	h2, err := NewClusterHandler(h.conf)
	assert.Nil(t, err)
	assert.NotNil(t, h2)
}

/*
TestSelectChild test clusterHandler.selectChild func.
route used as following:
root
- c1
	- c2
	- c3
- c4
	- c5
*/
func TestSelectChild(t *testing.T) {
	clusterrouter.Router().AddRoute("c1", "c1")
	clusterrouter.Router().AddRoute("c2", "c1")
	clusterrouter.Router().AddRoute("c3", "c1")
	clusterrouter.Router().AddRoute("c4", "c4")
	clusterrouter.Router().AddRoute("c5", "c4")

	cc := &otev1.ClusterController{
		Spec: otev1.ClusterControllerSpec{
			ClusterSelector: "c3,c5",
		},
	}
	selected := selectChild(cc)
	assert.Equal(t, 2, len(selected))
	assert.Equal(t, "c3", selected["c1"].Spec.ClusterSelector)
	assert.Equal(t, "c5", selected["c4"].Spec.ClusterSelector)
}

func TestHasToProcessClusterController(t *testing.T) {
	now := time.Now().Unix()
	cc := &otev1.ClusterController{
		ObjectMeta: metav1.ObjectMeta{
			CreationTimestamp: metav1.NewTime(time.Unix(now-1*60*60, 0)),
		},
	}
	assert.False(t, hasToProcessClusterController(cc))

	cc.ObjectMeta.CreationTimestamp = metav1.NewTime(time.Unix(now, 0))
	assert.True(t, hasToProcessClusterController(cc))

	cc.Status = map[string]otev1.ClusterControllerStatus{
		"c1": otev1.ClusterControllerStatus{},
	}
	assert.False(t, hasToProcessClusterController(cc))
}

func TestSendToChild(t *testing.T) {
	c := &clusterHandler{}
	fakeTunn.reset()
	c.tunn = fakeTunn
	c.sendToChild(nil)
	assert.False(t, fakeTunn.broadcastCalled)
	assert.False(t, fakeTunn.sendCalled)
	c.sendToChild(&otev1.ClusterController{})
	time.Sleep(1 * time.Second)
	assert.True(t, fakeTunn.broadcastCalled)
	assert.False(t, fakeTunn.sendCalled)
	fakeTunn.reset()
	c.sendToChild(&otev1.ClusterController{}, "")
	time.Sleep(1 * time.Second)
	assert.False(t, fakeTunn.broadcastCalled)
	assert.True(t, fakeTunn.sendCalled)
}

//func TestAddClusterController(t *testing.T) {
//	c := &clusterHandler{
//		tunn: newFakeCloudTunnel(),
//	}
//	cc := &otev1.ClusterController{
//		ObjectMeta: metav1.ObjectMeta{
//			CreationTimestamp: metav1.NewTime(time.Now()),
//		},
//	}
//	c.addClusterController()
//}

func TestStart(t *testing.T) {
	fakeK8sClient := oteclient.NewSimpleClientset()
	fakeTunn.reset()
	c := &clusterHandler{
		conf: &config.ClusterControllerConfig{
			TunnelListenAddr:      "fake",
			K8sClient:             fakeK8sClient,
			EdgeToClusterChan:     make(chan otev1.ClusterController),
			ClusterToEdgeChan:     make(chan otev1.ClusterController),
			ClusterUserDefineName: config.ROOT_CLUSTER_NAME,
		},
		tunn:      fakeTunn,
		k8sEnable: false,
	}
	err := c.valid()
	assert.Nil(t, err)
	err = c.Start()
	assert.Nil(t, err)

	// test msg from parent
	cc1 := otev1.ClusterController{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "cc1",
		},
		Spec: otev1.ClusterControllerSpec{
			ClusterSelector: "c1",
		},
	}
	clusterrouter.Router().AddRoute("c1", "c1")
	c.conf.EdgeToClusterChan <- cc1
	time.Sleep(1 * time.Second)
	assert.False(t, fakeTunn.broadcastCalled)
	assert.True(t, fakeTunn.sendCalled)
}

func TestAfterClusterConnect(t *testing.T) {
	c := &clusterHandler{
		tunn: fakeTunn,
	}
	cr := &config.ClusterRegistry{
		Name: "c1",
	}
	clusterrouter.Router().DelChild("c1", c.sendToChild)
	fakeTunn.reset()
	c.afterClusterConnect(cr)
	time.Sleep(1 * time.Second)
	assert.True(t, fakeTunn.broadcastCalled)
	assert.False(t, fakeTunn.sendCalled)
}

func TestHandleMessageFromChild(t *testing.T) {
	c := newFakeRootClusterHandler(t)

	err := c.handleMessageFromChild("c1", []byte("hahaha"))
	assert.NotNil(t, err)

	cc := otev1.ClusterController{}
	// regist cluster without cluster info
	cc.Spec.Destination = otev1.CLUSTER_CONTROLLER_DEST_REGIST_CLUSTER
	ccbytes, err := cc.Serialize()
	assert.Nil(t, err)
	err = c.handleMessageFromChild("c1", ccbytes)
	assert.NotNil(t, err)
	// unregist cluster without cluster info
	cc.Spec.Destination = otev1.CLUSTER_CONTROLLER_DEST_UNREGIST_CLUSTER
	ccbytes, err = cc.Serialize()
	assert.Nil(t, err)
	err = c.handleMessageFromChild("c1", ccbytes)
	assert.NotNil(t, err)
	// resp from child with no namespace and name
	cc.Spec.ParentClusterName = c.conf.ClusterUserDefineName
	cc.Spec.Destination = otev1.CLUSTER_CONTROLLER_DEST_API
	ccbytes, err = cc.Serialize()
	assert.Nil(t, err)
	err = c.handleMessageFromChild("c1", ccbytes)
	assert.Nil(t, err)
	// resp from child transmit to parent
	cc.Spec.ParentClusterName = c.conf.ClusterUserDefineName + "1"
	ccbytes, err = cc.Serialize()
	assert.Nil(t, err)
	err = c.handleMessageFromChild("c1", ccbytes)
	assert.Nil(t, err)
}

func TestHandleRegistClusterMessage(t *testing.T) {
	//croot := newFakeRootClusterHandler(t)

	//err := croot.handleRegistClusterMessage()
}

type fakeCloudTunnel struct {
	broadcastCalled bool
	sendCalled      bool
}

func newFakeCloudTunnel() *fakeCloudTunnel {
	return &fakeCloudTunnel{
		broadcastCalled: false,
		sendCalled:      false,
	}
}

func (f *fakeCloudTunnel) reset() {
	f.broadcastCalled = false
	f.sendCalled = false
}

func (f *fakeCloudTunnel) Start() error {
	return nil
}

func (f *fakeCloudTunnel) Stop() error {
	return nil
}

func (f *fakeCloudTunnel) Send(clusterName string, msg []byte) error {
	f.sendCalled = true
	return nil
}

func (f *fakeCloudTunnel) Broadcast(msg []byte) {
	f.broadcastCalled = true
}

func (f *fakeCloudTunnel) RegistCheckNameValidFunc(fn tunnel.ClusterNameChecker) {}

func (f *fakeCloudTunnel) RegistAfterConnectHook(fn tunnel.AfterConnectHook) {}

func (f *fakeCloudTunnel) RegistReturnMessageFunc(fn tunnel.TunnelReadMessageFunc) {}

func (f *fakeCloudTunnel) RegistClientCloseHandler(fn tunnel.ClientCloseHandleFunc) {}

func newFakeRootClusterHandler(t *testing.T) *clusterHandler {
	ret := &clusterHandler{
		conf: &config.ClusterControllerConfig{
			ClusterUserDefineName: config.ROOT_CLUSTER_NAME,
			TunnelListenAddr:      "8272",
			K8sClient:             oteclient.NewSimpleClientset(),
		},
		tunn:      fakeTunn,
		k8sEnable: false,
	}
	err := ret.valid()
	assert.Nil(t, err)
	fakeTunn.reset()
	return ret
}

func newFakeNoRootClusterHandler(t *testing.T) *clusterHandler {
	ret := &clusterHandler{
		conf: &config.ClusterControllerConfig{
			ClusterUserDefineName: "c1",
			TunnelListenAddr:      "8273",
			K8sClient:             oteclient.NewSimpleClientset(),
			ParentCluster:         "8272",
		},
		tunn:      fakeTunn,
		k8sEnable: false,
	}
	err := ret.valid()
	assert.Nil(t, err)
	fakeTunn.reset()
	return ret
}
