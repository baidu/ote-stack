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
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gorilla/websocket"
)

var (
	testServer *httptest.Server
)

func initTestServer() {
	testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader = websocket.Upgrader{}
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		for {
			mt, message, err := c.ReadMessage()
			if err != nil {
				break
			}
			err = c.WriteMessage(mt, message)
			if err != nil {
				break
			}
		}
	}))
}

func newTestWSClient() *WSClient {
	u := url.URL{Scheme: "ws", Host: testServer.Listener.Addr().String(), Path: "/"}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil
	}
	return NewWSClient("test", conn)
}

func TestWriteMessage(t *testing.T) {
	client := newTestWSClient()
	if client == nil {
		t.Errorf("can not build websocket connection")
	}

	expectMsg := "test msg"
	err := client.WriteMessage([]byte(expectMsg))
	if err != nil {
		t.Errorf("fail to send msg, err: %v", err)
		return
	}

}

func TestReadMessage(t *testing.T) {
	client := newTestWSClient()
	if client == nil {
		t.Errorf("can not build websocket connection")
	}

	// test receive
	expectMsg := "test msg"
	client.Conn.WriteMessage(websocket.BinaryMessage, []byte(expectMsg))
	msg, err := client.ReadMessage()
	if err != nil {
		t.Errorf("fail to read msg, err: %v", err)
	} else if string(msg) != expectMsg {
		t.Errorf("receive msg %s, expect %s", string(msg), expectMsg)
	}

	// test error
	client.Close()
	_, err = client.ReadMessage()
	if err == nil {
		t.Errorf("expect error")
	}
}

func TestMain(m *testing.M) {
	initTestServer()
	m.Run()
	testServer.Close()
}
