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
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/clustermessage"
)

const (
	ControllerSendChanBufferSize = 100
)

// ControllerTunnel is a iterface for controllerTunnel.
type ControllerTunnel interface {
	// Start will start controllerTunnel.
	Start() error
	// Stop will close the connection of controllerTunnel.
	Stop() error
	// Send sends binary message to websocket connection.
	Send(msg []byte) error
	SendChan() chan clustermessage.ClusterMessage
	// Regist registers receive message handler.
	RegistReceiveMessageHandler(TunnelReadMessageFunc)
	RegistAfterConnectToHook(fn AfterConnectToHook)
}

// controllerTunnel is responsible for communication with cloudTunnel.
type controllerTunnel struct {
	cloudAddr string
	wsclient  *WSClient

	receiveMessageHandler TunnelReadMessageFunc
	afterConnectToHook    AfterConnectToHook

	sendChan chan clustermessage.ClusterMessage
}

// NewControllerTunnel returns a new controllerTunnel object.
func NewControllerTunnel(remoteAddr string) ControllerTunnel {
	return &controllerTunnel{
		cloudAddr: remoteAddr,
		receiveMessageHandler: func(client string, msg []byte) error {
			fmt.Println(string(msg))
			return nil
		},
		afterConnectToHook: func() {},
		sendChan: make(chan clustermessage.ClusterMessage,
			ControllerSendChanBufferSize),
	}

}

func (e *controllerTunnel) connect() error {
	u := url.URL{Scheme: "ws", Host: e.cloudAddr, Path: controllerURI}
	header := http.Header{}

	klog.Infof("connecting to cloudtunnel %s", u.String())
	// TODO https connection.
	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), header)
	if err != nil {
		if resp != nil {
			klog.Errorf("failed to connect to cloudtunnel, code=%v", resp.StatusCode)
		}
		return err
	}

	// TODO gradeful new wsclient.
	e.wsclient = NewWSClient(e.cloudAddr, conn)

	go e.afterConnectToHook()

	return nil
}

func (e *controllerTunnel) Send(msg []byte) error {
	if e.wsclient == nil {
		return fmt.Errorf("controller tunnel is not ready")
	}
	err := e.wsclient.WriteMessage(msg)
	if err != nil {
		klog.Errorf("wsclient write msg failed: %s", err.Error())
		return err
	}
	return nil
}

func (e *controllerTunnel) SendChan() chan clustermessage.ClusterMessage {
	return e.sendChan
}

func (e *controllerTunnel) RegistReceiveMessageHandler(fn TunnelReadMessageFunc) {
	e.receiveMessageHandler = fn
}

func (e *controllerTunnel) RegistAfterConnectToHook(fn AfterConnectToHook) {
	e.afterConnectToHook = fn
}

func (e *controllerTunnel) Stop() error {
	//TODO: graceful stop.
	return nil
}

func (e *controllerTunnel) reconnect() {
	for {
		if err := e.connect(); err != nil {
			klog.Errorf("connect to %s failed, try again after %ds: %s",
				e.cloudAddr, waitConnection, err.Error())
			time.Sleep(waitConnection * time.Second)
			continue
		}
		break
	}
}
func (e *controllerTunnel) Start() error {
	if err := e.connect(); err != nil {
		return err
	}

	// TODO exit if name is duplicate.
	go func() {
		for {
			// send from chan
			go e.sendFromChan()
			// read from websocket
			e.handleReceiveMessage()

			e.wsclient.Close()
			e.reconnect()
		}
	}()
	return nil
}

func (e *controllerTunnel) sendFromChan() {
	// TODO get msg from sendChan and send it out
}

func (e *controllerTunnel) handleReceiveMessage() {
	klog.V(1).Infof("start handle receive message")
	for {
		msg, err := e.wsclient.ReadMessage()
		if err != nil {
			klog.Errorf("read msg failed: %s", err.Error())
			break
		}

		e.receiveMessageHandler(e.wsclient.Name, msg)
	}
}
