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
	"container/list"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"k8s.io/klog"

	clusterrouter "github.com/baidu/ote-stack/pkg/clusterrouter"
	"github.com/baidu/ote-stack/pkg/config"
)

var (
	waitConnection   = 1
	blacklistSeconds = 10
)

// EdgeTunnel is a iterface for edgeTunnel.
type EdgeTunnel interface {
	// Start will start edgeTunnel.
	Start() error
	// Stop will close the connection of edgeTunnel.
	Stop() error
	// Send sends binary message to websocket connection.
	Send(msg []byte) error
	// Regist registers receive message handler.
	RegistReceiveMessageHandler(TunnelReadMessageFunc)
	RegistAfterConnectToHook(fn AfterConnectToHook)
	RegistAfterDisconnectHook(fn AfterDisconnectHook)
}

// edgeTunnel is responsible for communication with cloudTunnel.
type edgeTunnel struct {
	conf       *config.ClusterControllerConfig
	cloudAddr  string
	name       string
	uuid       string
	listenAddr string
	wsclient   *WSClient

	receiveMessageHandler TunnelReadMessageFunc
	afterConnectToHook    AfterConnectToHook
	afterDisconnectHook   AfterDisconnectHook
}

// NewEdgeTunnel returns a new edgeTunnel object.
func NewEdgeTunnel(conf *config.ClusterControllerConfig) EdgeTunnel {
	return &edgeTunnel{
		conf:       conf,
		name:       conf.ClusterUserDefineName,
		cloudAddr:  conf.ParentCluster,
		listenAddr: conf.TunnelListenAddr,
		receiveMessageHandler: func(client string, msg []byte) error {
			klog.Info(string(msg))
			return nil
		},
		afterConnectToHook:  func() {},
		afterDisconnectHook: func() {},
	}

}

func (e *edgeTunnel) connect() error {
	e.uuid = e.name
	u := url.URL{Scheme: "ws", Host: e.cloudAddr, Path: accessURI + e.uuid}
	header := http.Header{}
	header.Add(config.ClusterConnectHeaderListenAddr, e.listenAddr)
	header.Add(config.ClusterConnectHeaderUserDefineName, e.name)

	klog.Infof("connecting to cloudtunnel %s", u.String())
	// TODO https connection.
	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), header)
	if err != nil {
		if resp != nil {
			klog.Errorf("failed to connect to cloudtunnel, code=%v", resp.StatusCode)
		}
		return err
	}

	e.conf.ClusterName = e.uuid

	// TODO gradeful new wsclient.
	e.wsclient = NewWSClient(e.uuid, conn)

	go e.afterConnectToHook()

	return nil
}

func (e *edgeTunnel) Send(msg []byte) error {
	if e.wsclient == nil {
		return fmt.Errorf("edge tunnel is not ready")
	}
	err := e.wsclient.WriteMessage(msg)
	if err != nil {
		klog.Errorf("wsclient write msg failed: %s", err.Error())
		return err
	}
	return nil
}

func (e *edgeTunnel) RegistReceiveMessageHandler(fn TunnelReadMessageFunc) {
	e.receiveMessageHandler = fn
}

func (e *edgeTunnel) RegistAfterConnectToHook(fn AfterConnectToHook) {
	e.afterConnectToHook = fn
}

func (e *edgeTunnel) RegistAfterDisconnectHook(fn AfterDisconnectHook) {
	e.afterDisconnectHook = fn
}

func (e *edgeTunnel) Stop() error {
	//TODO: graceful stop.
	return nil
}

func (e *edgeTunnel) reconnect() {
	for {
		if err := e.connect(); err != nil {
			// if disconnect to parent, choose a parent neighbor to connect.
			if !e.chooseParentNeighbor() {
				// wait and connect to current parent.
				klog.Errorf("connect to %s failed, try again after %ds: %s",
					e.cloudAddr, waitConnection, err.Error())
				time.Sleep(time.Duration(waitConnection) * time.Second)
			}

			// connect to new parent immediately.
			klog.Errorf("connect to new parent %s", e.cloudAddr)
			continue
		}
		break
	}

	// cloud address is not needed in black list after connecting to a parent.
	defaultCloudBlackList.Clear()
}
func (e *edgeTunnel) Start() error {
	if err := e.connect(); err != nil {
		return err
	}

	// TODO exit if name is duplicate.
	go func() {
		for {
			e.handleReceiveMessage()

			e.wsclient.Close()
			e.reconnect()
		}
	}()
	return nil
}

// handleReceiveMessage reads message from the connection and process it one by one.
// this function will block until error occurs in the connection,
// and once error happened, call afterDisconnectHook immediately
func (e *edgeTunnel) handleReceiveMessage() {
	klog.V(1).Infof("start handle receive message")
	for {
		msg, err := e.wsclient.ReadMessage()
		if err != nil {
			klog.Errorf("read msg failed: %s", err.Error())
			break
		}

		e.receiveMessageHandler(e.wsclient.Name, msg)
	}
	klog.Warningf("disconnect from %s", e.cloudAddr)
	e.afterDisconnectHook()
}

// chooseParentNeighbor change cloud addrrss of edge tunnel
// if a parent or neighbor node found.
func (e *edgeTunnel) chooseParentNeighbor() bool {
	// push current parent to blacklist.
	defaultCloudBlackList.Push(e.cloudAddr)
	// find first parent neighbor not in blacklist.
	var choose string
	// TODO choose from parent neighbor or parent's parent.
	for _, addr := range clusterrouter.Router().ParentNeighbors() {
		if !defaultCloudBlackList.Find(addr) {
			choose = addr
			break
		}
	}
	if choose == "" {
		// pop from blacklist
		choose = defaultCloudBlackList.Pop()
	}

	if choose == "" {
		klog.Errorf("cannot find a parent neighbor to connect, do not change parent")
		return false
	}

	e.cloudAddr = choose
	return true
}

/*
CloudBlackList records cloud address to blacklist.

Once disconnect from a parent,
child use parent neighbor which out of blacklist to reconnect.
If no parent neighbor available,
get one from blacklist use FIFO for blacklist.
*/
type CloudBlackList struct {
	// for FIFO.
	addrList *list.List
	// for o(1) find, addr->unix time added.
	addrMap map[string]time.Time
}

var defaultCloudBlackList = CloudBlackList{
	addrList: list.New(),
	addrMap:  make(map[string]time.Time),
}

// Push a cloud address to list.
func (c *CloudBlackList) Push(addr string) {
	if _, ok := c.addrMap[addr]; ok {
		// addr already in blacklist.
		return
	}

	c.addrList.PushBack(addr)
	c.addrMap[addr] = time.Now()
}

// Pop the front cloud address from list.
func (c *CloudBlackList) Pop() string {
	if c.addrList.Len() == 0 {
		return ""
	}

	first := c.addrList.Front()
	addr := first.Value.(string)
	timeAdded := c.addrMap[addr]
	if time.Since(timeAdded) <= time.Duration(blacklistSeconds)*time.Second {
		return ""
	}
	c.addrList.Remove(first)
	delete(c.addrMap, addr)
	return addr
}

// Find whether the address is in the list.
func (c *CloudBlackList) Find(key string) bool {
	_, ok := c.addrMap[key]
	return ok
}

// Clear all cloud address in the list.
func (c *CloudBlackList) Clear() {
	c.addrList.Init()

	for key := range c.addrMap {
		delete(c.addrMap, key)
	}
}
