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

package edgehandler

import (
	"reflect"
	"testing"
	"time"

	otev1 "github.com/baidu/ote-stack/pkg/apis/ote/v1"
	shimv1 "github.com/baidu/ote-stack/pkg/clustershim/apis/v1"
	"github.com/baidu/ote-stack/pkg/clustershim/handler"
	"github.com/baidu/ote-stack/pkg/config"
	oteclient "github.com/baidu/ote-stack/pkg/generated/clientset/versioned"
	"github.com/baidu/ote-stack/pkg/tunnel"
)

var LastSend otev1.ClusterController

type fakeEdgeTunnel struct {
}

type fakeShimHandler struct {
}

func (f *fakeEdgeTunnel) Send(msg []byte) error {
	cc, err := otev1.ClusterControllerDeserialize(msg)
	if err != nil {
		return err
	}
	LastSend = *cc
	return nil
}

func (f *fakeEdgeTunnel) RegistReceiveMessageHandler(tunnel.TunnelReadMessageFunc) {
	return
}

func (f *fakeEdgeTunnel) RegistAfterConnectToHook(tunnel.AfterConnectToHook) {
	return
}

func (f *fakeEdgeTunnel) Start() error {
	return nil
}

func (f *fakeEdgeTunnel) Stop() error {
	return nil
}

func (f *fakeShimHandler) Do(in *shimv1.ShimRequest) (*shimv1.ShimResponse, error) {
	resp := &shimv1.ShimResponse{
		Timestamp:  time.Now().Unix(),
		StatusCode: 200,
		Body:       "",
	}
	return resp, nil
}

func newFakeShim() shimServiceClient {
	local := &localShimClient{
		handlers: make(map[string]handler.Handler),
	}
	local.handlers[otev1.ClusterControllerDestAPI] = &fakeShimHandler{}
	return local
}

func TestValid(t *testing.T) {
	succescase := []struct {
		Name string
		Conf *config.ClusterControllerConfig
	}{
		{
			Name: "edgehandler with k8sclient",
			Conf: &config.ClusterControllerConfig{
				ClusterName:           "child",
				ClusterUserDefineName: "child",
				K8sClient:             &oteclient.Clientset{},
				RemoteShimAddr:        "",
				ParentCluster:         "127.0.0.1:8287",
			},
		},
		{
			Name: "edgehandler with remoteshim",
			Conf: &config.ClusterControllerConfig{
				ClusterName:           "child",
				ClusterUserDefineName: "child",
				K8sClient:             nil,
				RemoteShimAddr:        "/var/run/shim.sock",
				ParentCluster:         "127.0.0.1:8287",
			},
		},
	}

	for _, sc := range succescase {
		edge := edgeHandler{conf: sc.Conf}
		if err := edge.valid(); err != nil {
			t.Errorf("[%q] unexpected error %v", sc.Name, err)
		}
	}

	errorcase := []struct {
		Name string
		Conf *config.ClusterControllerConfig
	}{
		{
			Name: "cluster name not set",
			Conf: &config.ClusterControllerConfig{
				ClusterName:    "",
				K8sClient:      nil,
				RemoteShimAddr: "/var/run/shim.sock",
				ParentCluster:  "127.0.0.1:8287",
			},
		},
		{
			Name: "shim address not set",
			Conf: &config.ClusterControllerConfig{
				ClusterName:    "child1",
				K8sClient:      nil,
				RemoteShimAddr: "",
				ParentCluster:  "127.0.0.1:8287",
			},
		},
		{
			Name: "ParentCluster not set",
			Conf: &config.ClusterControllerConfig{
				ClusterName:    "child1",
				K8sClient:      nil,
				RemoteShimAddr: "/var/run/shim.sock",
				ParentCluster:  "",
			},
		},
	}

	for _, ec := range errorcase {
		edge := &edgeHandler{conf: ec.Conf}
		if err := edge.valid(); err == nil {
			t.Errorf("[%q] expected error", ec.Name)
		}
	}
}

func TestIsRemoteShim(t *testing.T) {
	casetest := []struct {
		Name   string
		Conf   *config.ClusterControllerConfig
		Expect bool
	}{
		{
			Name: "use remote shim",
			Conf: &config.ClusterControllerConfig{
				ClusterName:    "child",
				RemoteShimAddr: "/var/run/shim.sock",
				K8sClient:      &oteclient.Clientset{},
			},
			Expect: true,
		},
		{
			Name: "use local shim",
			Conf: &config.ClusterControllerConfig{
				ClusterName:    "child",
				RemoteShimAddr: "",
				K8sClient:      &oteclient.Clientset{},
			},
			Expect: false,
		},
	}
	for _, ct := range casetest {
		edge := &edgeHandler{
			conf: ct.Conf,
		}
		res := edge.isRemoteShim()
		if res != ct.Expect {
			t.Errorf("[%q] expected %v, got %v", ct.Name, ct.Expect, res)
		}
	}
}

func TestSendMessageToTunnel(t *testing.T) {
	conf := &config.ClusterControllerConfig{
		ClusterName:       "child",
		K8sClient:         nil,
		RemoteShimAddr:    "/var/run/shim.sock",
		ParentCluster:     "127.0.0.1:8287",
		ClusterToEdgeChan: make(chan otev1.ClusterController),
	}

	casetest := []struct {
		Name     string
		SendData otev1.ClusterController
	}{
		{
			Name: "valid send clusterController",
			SendData: otev1.ClusterController{
				Spec: otev1.ClusterControllerSpec{
					ParentClusterName: "root",
					ClusterSelector:   "c1,c2",
					Destination:       "api",
				},
			},
		},
	}

	for _, ct := range casetest {
		edge := &edgeHandler{
			conf:       conf,
			edgeTunnel: &fakeEdgeTunnel{},
		}
		go edge.sendMessageToTunnel()
		edge.conf.ClusterToEdgeChan <- ct.SendData
		time.Sleep(1 * time.Second)
		if !reflect.DeepEqual(ct.SendData, LastSend) {
			t.Errorf("[%q] expected %v, got %v", ct.Name, ct.SendData, LastSend)
		}
	}
}

func TestReceiveMessageFromTunnel(t *testing.T) {
	conf := &config.ClusterControllerConfig{
		ClusterName:       "child",
		K8sClient:         nil,
		RemoteShimAddr:    "/var/run/shim.sock",
		ParentCluster:     "127.0.0.1:8287",
		EdgeToClusterChan: make(chan otev1.ClusterController, 10),
	}

	edge := &edgeHandler{
		conf:       conf,
		edgeTunnel: &fakeEdgeTunnel{},
		shimClient: newFakeShim(),
	}

	casetest := []struct {
		Name         string
		Data         otev1.ClusterController
		ExpectHandle bool
	}{
		{
			Name: "match rule",
			Data: otev1.ClusterController{
				Spec: otev1.ClusterControllerSpec{
					ParentClusterName: "root",
					ClusterSelector:   "c1,c2,child",
					Destination:       "api",
				},
			},
			ExpectHandle: true,
		},
		{
			Name: "not match rule",
			Data: otev1.ClusterController{
				Spec: otev1.ClusterControllerSpec{
					ParentClusterName: "root",
					ClusterSelector:   "c1,c2",
					Destination:       "api",
				},
			},
			ExpectHandle: false,
		},
	}

	for _, ct := range casetest {
		LastSend = otev1.ClusterController{}
		msg, _ := ct.Data.Serialize()
		edge.receiveMessageFromTunnel(conf.ClusterName, msg)

		var broadcast otev1.ClusterController
		go func() {
			broadcast = <-edge.conf.EdgeToClusterChan
		}()

		time.Sleep(1 * time.Second)

		_, ok := LastSend.Status[conf.ClusterName]
		if ct.ExpectHandle && !ok {
			t.Errorf("[%q] expected handle msg", ct.Name)
		} else if !ct.ExpectHandle && ok {
			t.Errorf("[%q] expected not handle msg", ct.Name)
		}

		if !reflect.DeepEqual(ct.Data, broadcast) {
			t.Errorf("[%q] expected %v, got %v", ct.Name, ct.Data, broadcast)
		}
	}
}

func TestHandleMessage(t *testing.T) {
	conf := &config.ClusterControllerConfig{
		ClusterName:       "child",
		K8sClient:         nil,
		RemoteShimAddr:    "/var/run/shim.sock",
		ParentCluster:     "127.0.0.1:8287",
		EdgeToClusterChan: make(chan otev1.ClusterController, 10),
	}
	edge := &edgeHandler{
		conf:       conf,
		edgeTunnel: &fakeEdgeTunnel{},
		shimClient: newFakeShim(),
	}

	casetest := []struct {
		Name         string
		Data         otev1.ClusterController
		ExpectCode   int
		ExpectHandle bool
	}{
		{
			Name: "dispatch to route",
			Data: otev1.ClusterController{
				Spec: otev1.ClusterControllerSpec{
					ParentClusterName: "root",
					Destination:       otev1.ClusterControllerDestClusterRoute,
				},
			},
			ExpectCode:   0,
			ExpectHandle: false,
		},
		{
			Name: "dispatch to api",
			Data: otev1.ClusterController{
				Spec: otev1.ClusterControllerSpec{
					ParentClusterName: "root",
					Destination:       otev1.ClusterControllerDestAPI,
				},
			},
			ExpectCode:   200,
			ExpectHandle: true,
		},
		{
			Name: "dispatch to api",
			Data: otev1.ClusterController{
				Spec: otev1.ClusterControllerSpec{
					ParentClusterName: "root",
					Destination:       otev1.ClusterControllerDestHelm,
				},
			},
			ExpectCode:   500,
			ExpectHandle: true,
		},
	}

	for _, ct := range casetest {
		LastSend = otev1.ClusterController{}
		if err := edge.handleMessage(&ct.Data); err != nil {
			t.Errorf("[%q] unexpected error %v", ct.Name, err)
		}

		time.Sleep(2 * time.Second)
		status, ok := LastSend.Status[conf.ClusterName]
		if !ct.ExpectHandle && ok {
			t.Errorf("[%q] expected not handle msg", ct.Name)
		}

		if ct.ExpectHandle {
			if !ok {
				t.Errorf("[%q] expected handle msg", ct.Name)
			} else if status.StatusCode != ct.ExpectCode {
				t.Errorf("[%q] expected %v, got %v",
					ct.Name, ct.ExpectCode, status.StatusCode)
			}

		}
	}

}
