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
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"

	"github.com/baidu/ote-stack/pkg/clustermessage"
)

var (
	testServerWillStop        *httptest.Server
	testConnectionClosed      = false
	testConnectionClosedCond  = sync.NewCond(&sync.Mutex{})
	testConnectionOnce        = sync.Once{}
	testConnectionGetMsgCount = 0
)

func initTestServerWillStop() {
	testServerWillStop = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader = websocket.Upgrader{}
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		fmt.Println("connection established")
		for {
			_, _, err := c.ReadMessage()
			if err != nil {
				break
			}
			testConnectionGetMsgCount++
			fmt.Printf("get a msg %d\n", testConnectionGetMsgCount)
			testConnectionOnce.Do(func() {
				fmt.Println("connection will be closed")
				c.Close()
				testConnectionClosed = true
				testConnectionClosedCond.Signal()
			})
		}
	}))
}

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

func TestControllerTunnelInterface(t *testing.T) {
	initTestServerWillStop()
	testServerWillStop.Start()
	// make a tunnel with 10 buffer size and 3 second reconnect interval, and connect it to server
	ControllerSendChanBufferSize = 1
	waitConnection = 1
	tun := NewControllerTunnel(testServerWillStop.Listener.Addr().String()).(*controllerTunnel)
	tun.Start()

	// lock connection health cond
	tun.connectionHealthCond.L.Lock()

	// send msg to server with no block
	sendChan := tun.SendChan()
	sendChan <- clustermessage.ClusterMessage{}
	stopChan := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopChan:
				break
			default:
				sendChan <- clustermessage.ClusterMessage{}
			}
		}
	}()

	testConnectionClosedCond.L.Lock()
	for !testConnectionClosed {
		testConnectionClosedCond.Wait()
	}
	testConnectionClosedCond.L.Unlock()

	time.Sleep(1 * time.Second)
	assert.False(t, tun.connectionHealth)
	// unlock to reconnect
	tun.connectionHealthCond.L.Unlock()
	close(stopChan)
	// wait reconnect
	time.Sleep(1 * time.Second)
	// after reconnect
	assert.True(t, tun.connectionHealth)
}
