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
	"sync"
	"time"

	proto "github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/clustermessage"
)

var (
	ControllerSendChanBufferSize = 1000
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
	cloudAddr       string
	originCloudAddr string // set to setting cloud addr when redirect to another
	wsclient        *WSClient

	receiveMessageHandler TunnelReadMessageFunc
	afterConnectToHook    AfterConnectToHook

	sendChan chan clustermessage.ClusterMessage

	connectionHealth     bool
	connectionHealthCond *sync.Cond
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
		connectionHealth:     false,
		connectionHealthCond: sync.NewCond(&sync.Mutex{}),
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
			if resp.StatusCode == http.StatusFound {
				redirectLocation, err := resp.Location()
				if err != nil {
					klog.Errorf("failed to redirect, err=%v", err)
					return err
				}
				klog.Infof("redirect to %s", redirectLocation.String())
				e.originCloudAddr = e.cloudAddr
				e.cloudAddr = redirectLocation.Host
				return e.connect()
			}
			klog.Errorf("failed to connect to cloudtunnel, code=%v", resp.StatusCode)
		}
		return err
	}

	// TODO gradeful new wsclient.
	e.wsclient = NewWSClient(e.cloudAddr, conn)

	go e.afterConnectToHook()

	return nil
}

// TODO do sth if sends failed
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
		e.connectionHealthCond.L.Lock()
		e.connectionHealth = false
		if err := e.connect(); err != nil {
			// if it has be redirected, try the origin parent first
			if e.originCloudAddr != "" {
				// wait for leader elect
				time.Sleep(1 * time.Second)

				klog.Infof("reconnect to origin parent %s", e.originCloudAddr)
				e.cloudAddr = e.originCloudAddr
				e.originCloudAddr = ""
				e.connectionHealthCond.L.Unlock()
				e.connectionHealthCond.Signal()
				continue
			}
			klog.Errorf("connect to %s failed, try again after %ds: %s",
				e.cloudAddr, waitConnection, err.Error())
			e.connectionHealthCond.L.Unlock()
			e.connectionHealthCond.Signal()
			time.Sleep(time.Duration(waitConnection) * time.Second)
			continue
		}
		e.connectionHealth = true
		e.connectionHealthCond.L.Unlock()
		e.connectionHealthCond.Signal()
		break
	}
}
func (e *controllerTunnel) Start() error {
	if err := e.connect(); err != nil {
		return err
	}
	e.connectionHealth = true
	go func() {
		// send from chan
		go e.sendFromChan()
		for {
			// read from websocket
			e.handleReceiveMessage()

			klog.Errorf("connection to %s failed, try to reconnect", e.cloudAddr)

			e.wsclient.Close()
			e.reconnect()
		}
	}()
	return nil
}

func (e *controllerTunnel) sendFromChan() {
	// get msg from sendChan and send it out
	var msg clustermessage.ClusterMessage
	for {
		msg = <-e.sendChan
		data, err := proto.Marshal(&msg)
		if err != nil {
			klog.Errorf("serialize cluster message(%v) failed: %v", msg, err)
			continue
		}
		err = e.Send(data)
		if err == nil {
			continue
		}
		klog.Errorf("send msg failed: %v", err)
		e.connectionHealth = false
		err = e.recoverAndReSend(data)
		if err != nil {
			klog.Errorf("%v", err)
		}
	}
}

func (e *controllerTunnel) recoverAndReSend(resendData []byte) error {
	// wait until connection recover or send channel is full
	e.connectionHealthCond.L.Lock()
	for !e.connectionHealth {
		if len(e.sendChan) == ControllerSendChanBufferSize {
			e.connectionHealthCond.L.Unlock()
			return fmt.Errorf("send channel is full but connection is not health, throw the msg")
		}
		e.connectionHealthCond.Wait()
	}
	// resend msg if send error
	err := e.Send(resendData)
	if err == nil {
		e.connectionHealthCond.L.Unlock()
		return nil
	}
	e.connectionHealth = false
	e.connectionHealthCond.L.Unlock()
	return e.recoverAndReSend(resendData)
}

func (e *controllerTunnel) handleReceiveMessage() {
	klog.V(1).Infof("start handle receive message")
	for {
		msg, err := e.wsclient.ReadMessage()
		if err != nil {
			klog.Errorf("read msg failed: %s", err.Error())
			break
		}

		go e.receiveMessageHandler(e.wsclient.Name, msg)
	}
}
