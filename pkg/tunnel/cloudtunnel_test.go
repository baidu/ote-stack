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
	"net"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"

	"github.com/baidu/ote-stack/pkg/config"
)

var (
	ipAddrForTest = &net.IPAddr{[]byte{}, ""}
)

func TestRemoveFromSliceByValue(t *testing.T) {
	slice := []string{"a", "b", "c", "d"}
	// value not in slice
	r := removeFromSliceByValue(slice, "e")
	assert.ElementsMatch(t, slice, r)
	// value in slice
	r = removeFromSliceByValue(slice, "c")
	assert.ElementsMatch(t, r, []string{"a", "b", "d"})
}

func TestFindAController(t *testing.T) {
	clientMap := sync.Map{}
	var clientKey []string
	// clientKey is empty
	ws := findAController(clientMap, clientKey)
	assert.Nil(t, ws)
	// find one in clientKey, but not in clientMap, finally find nothing
	clientKey = append(clientKey, "a")
	ws = findAController(clientMap, clientKey)
	assert.Nil(t, ws)
	oWs := &WSClient{}
	clientMap.Store("a", oWs)
	ws = findAController(clientMap, clientKey)
	assert.NotNil(t, ws)
	assert.Equal(t, oWs, ws)
}

func TestListenedCloudTunnel(t *testing.T) {
	ctInter := NewCloudTunnel("")
	ct := ctInter.(*cloudTunnel)
	err := ct.Start()
	addr := ct.server.Addr
	clientName := "c1"
	u, err := url.Parse(fmt.Sprintf("ws://%s%s%s", addr, accessURI, clientName))
	header := http.Header{}
	header.Add(config.ClusterConnectHeaderListenAddr, "fake")
	header.Add(config.ClusterConnectHeaderUserDefineName, clientName)
	assert.Nil(t, err)
	// wait listen
	time.Sleep(1 * time.Second)
	assert.Nil(t, err)
	// connect a client
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), header)
	assert.NotNil(t, conn)
	assert.Nil(t, err)
	//TODO assert wsclient exits
	//	client1, ok := ct.clients.Load("c1")
	//	assert.True(t, ok)
	//	assert.NotNil(t, client1)

	// send to c1
	err = ct.Send(clientName, []byte{})
	assert.Nil(t, err)
	// send to not exist c2
	err = ct.Send("c2", []byte{})
	assert.NotNil(t, err)
	// send to controller manager while no one exist
	err = ct.SendToControllerManager([]byte{})
	assert.NotNil(t, err)
	// connect a controller
	u, err = url.Parse(fmt.Sprintf("ws://%s%s", addr, controllerURI))
	conn, _, err = websocket.DefaultDialer.Dial(u.String(), header)
	assert.NotNil(t, conn)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(ct.controllersKey))
	// send to controller manager
	err = ct.SendToControllerManager([]byte{})
	assert.Nil(t, err)
}
