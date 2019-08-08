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

// Package tunnel implements websocket communication between the cloud server and edge clients.
package tunnel

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/config"
)

const (
	WriteTimeout = time.Second * 15
	ReadTimeout  = time.Second * 15
	IdleTimeout  = time.Second * 60
	StopTimeout  = time.Second * 15
)

// WSClient is a websocket client.
type WSClient struct {
	// Name defines uuid of the client.
	Name string
	// Conn defines websocket connection.
	Conn  *websocket.Conn
	mutex sync.Mutex
}

// ClusterNameChecker is a function to check cluster name.
type ClusterNameChecker func(*config.ClusterRegistry) bool

// TunnelReadMessageFunc is a function to handle message from tunnel.
// this function takes 2 arguments, first mean client name(cluster name),
// the second is message data.
type TunnelReadMessageFunc func(string, []byte) error

// ClientCloseHandleFunc is a function to handle wsclient close event.
type ClientCloseHandleFunc func(*config.ClusterRegistry)

// AfterConnectHook is a function to handle wsclient connection event.
type AfterConnectHook func(*config.ClusterRegistry)

// AfterConnectToHook is a function of edge tunnel to call after connection established.
type AfterConnectToHook func()

// NewWSClient returns a websocket client.
func NewWSClient(name string, conn *websocket.Conn) *WSClient {
	wsclient := &WSClient{
		Name: name,
		Conn: conn,
	}
	return wsclient
}

// Close closes websocket connection.
func (c *WSClient) Close() error {
	return c.Conn.Close()
}

// WriteMessage writes binary message to connection.
func (c *WSClient) WriteMessage(msg []byte) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.Conn.SetWriteDeadline(time.Now().Add(WriteTimeout))
	if err := c.Conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
		klog.Errorf("wsclient %s write msg failed: %s", c.Name, err.Error())
		return err
	}

	return nil
}

// ReadMessage reads binary message from connection.
func (c *WSClient) ReadMessage() ([]byte, error) {
	_, message, err := c.Conn.ReadMessage()
	if err != nil {
		klog.Errorf("wsclient %s read msg failed: %s", c.Name, err.Error())
		return nil, err
	}
	return message, nil
}
