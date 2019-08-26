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
	"reflect"
	"testing"
	"time"
)

func newTestControllerTunnel() *controllerTunnel {
	return &controllerTunnel{
		cloudAddr:          testServer.Listener.Addr().String(),
		afterConnectToHook: func() {},
	}
}
func TestControllerTunnelConnect(t *testing.T) {
	tun := newTestControllerTunnel()

	if err := tun.connect(); err != nil {
		t.Errorf("connect unexpected error %v", err)
	}
}

func TestControllerTunnelSend(t *testing.T) {
	tun := newTestControllerTunnel()
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

func TestControllerTunnelHandleReceiveMessage(t *testing.T) {
	tun := newTestControllerTunnel()

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
