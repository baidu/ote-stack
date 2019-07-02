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

package tunnel

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	clusterrouter "github.com/baidu/ote-stack/pkg/clusterrouter"
)

func newTestEdgeTunnel() *edgeTunnel {
	return &edgeTunnel{
		name:       "child",
		cloudAddr:  testServer.Listener.Addr().String(),
		listenAddr: ":8287",
	}
}
func TestConnect(t *testing.T) {
	tun := newTestEdgeTunnel()

	if err := tun.connect(); err != nil {
		t.Errorf("connect unexpected error %v", err)
	}
}

func TestSend(t *testing.T) {
	tun := newTestEdgeTunnel()
	if err := tun.Send([]byte("test")); err == nil {
		t.Errorf("send expected error")
	}

	if err := tun.connect(); err != nil {
		t.Errorf("send unexpected error %v", err)
	}

	if err := tun.Send([]byte("test")); err != nil {
		t.Errorf("send unexpected error %v", err)
	}
}

func TestHandleReceiveMessage(t *testing.T) {
	tun := newTestEdgeTunnel()

	var lastmsg []byte
	readMessage := func(name string, msg []byte) error {
		lastmsg = msg
		return nil
	}

	tun.connect()
	tun.RegistReceiveMessageHandler(readMessage)
	go tun.handleReceiveMessage()

	casetest := []struct {
		Name    string
		SendMsg []byte
	}{
		{
			Name:    "handle message",
			SendMsg: []byte("test message"),
		},
	}

	for _, ct := range casetest {

		if err := tun.wsclient.WriteMessage(ct.SendMsg); err != nil {
			t.Errorf("[%q] unexpected error %v", ct.Name, err)
		}

		time.Sleep(1 * time.Second)
		if !reflect.DeepEqual(ct.SendMsg, lastmsg) {
			t.Errorf("[%q] expected %v, got %v", ct.Name, ct.SendMsg, lastmsg)
		}
	}
}

func TestChooseParentNeighbor(t *testing.T) {
	fmt.Printf("len:%v\n", defaultCloudBlackList.addrList.Len())
	tun := newTestEdgeTunnel()

	casetest := []struct {
		Name         string
		Parents      map[string]string
		ExpectParent string
		ExpectResult bool
	}{
		{
			Name: "test with one parent",
			Parents: map[string]string{
				"c1": testServer.Listener.Addr().String(),
			},
			ExpectParent: testServer.Listener.Addr().String(),
			ExpectResult: false,
		},
		{
			Name: "test with two parent",
			Parents: map[string]string{
				"c1": testServer.Listener.Addr().String(),
				"c2": "127.0.0.1:1234",
			},
			ExpectParent: "127.0.0.1:1234",
			ExpectResult: true,
		},
		{
			Name: "test with same parent again",
			Parents: map[string]string{
				"c1": testServer.Listener.Addr().String(),
				"c2": "127.0.0.1:1234",
			},
			ExpectParent: "127.0.0.1:1234",
			ExpectResult: false,
		},
	}

	for _, ct := range casetest {
		clusterrouter.Router().ParentNeighbor = ct.Parents
		result := tun.chooseParentNeighbor()
		if result != ct.ExpectResult {
			t.Errorf("[%q] expected %v, got %v", ct.Name, ct.ExpectResult, result)
		}
		if tun.cloudAddr != ct.ExpectParent {
			t.Errorf("[%q] expected %v, got %v", ct.Name, ct.ExpectParent, tun.cloudAddr)
		}
	}
}
