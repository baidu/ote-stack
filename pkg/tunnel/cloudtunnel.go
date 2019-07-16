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
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/config"
)

const (
	accessURI        = "/access/"
	accessURIParam   = "cluster_id"
	accessURIPattern = "/access/{%s}"
)

var upgrader = websocket.Upgrader{}

// CloudTunnel is interface for cloudtunnel.
type CloudTunnel interface {
	// Start will start cloudtunnel.
	Start() error
	// Stop will shut down the server and close all websocket connection.
	Stop() error
	// Send sends binary message to the given wsclient.
	Send(clusterName string, msg []byte) error
	// Broadcast sends binary message to all connected wsclient.
	Broadcast(msg []byte)
	// RegistCheckNameValidFunc registers ClusterNameChecker.
	RegistCheckNameValidFunc(fn ClusterNameChecker)
	// RegistAfterConnectHook registers AfterConnectHook.
	RegistAfterConnectHook(fn AfterConnectHook)
	// RegistReturnMessageFunc registers TunnelReadMessageFunc.
	RegistReturnMessageFunc(fn TunnelReadMessageFunc)
	// RegistClientCloseHandler registers ClientCloseHandleFunc.
	RegistClientCloseHandler(fn ClientCloseHandleFunc)
}

// cloudTunnel handles all communications with edgetunnel.
type cloudTunnel struct {
	clients               sync.Map
	address               string
	clusterNameCheck      ClusterNameChecker
	receiveMessageHandler TunnelReadMessageFunc
	notifyClientClosed    ClientCloseHandleFunc
	afterConnectHook      AfterConnectHook
}

// NewCloudTunnel returns a new cloudTunnel object.
func NewCloudTunnel(address string) CloudTunnel {
	tunnel := &cloudTunnel{
		address:            address,
		clusterNameCheck:   defaultClusterNameChecker,
		notifyClientClosed: func(*config.ClusterRegistry) { return },
		afterConnectHook:   defaultAfterConnectHook,
	}

	tunnel.receiveMessageHandler = func(client string, msg []byte) error {
		return nil
	}
	return tunnel
}

func (t *cloudTunnel) Broadcast(msg []byte) {
	broadcast := func(key, value interface{}) bool {
		client, ok := value.(*WSClient)
		if ok {
			go client.WriteMessage(msg)
		}
		return true
	}
	t.clients.Range(broadcast)
}

func (t *cloudTunnel) Send(clusterName string, msg []byte) error {
	client, ok := t.clients.Load(clusterName)
	if ok {
		wsclient := client.(*WSClient)
		return wsclient.WriteMessage(msg)
	}
	return fmt.Errorf("client %s not found", clusterName)
}

func (t *cloudTunnel) RegistCheckNameValidFunc(fn ClusterNameChecker) {
	t.clusterNameCheck = fn
}

func (t *cloudTunnel) RegistReturnMessageFunc(fn TunnelReadMessageFunc) {
	t.receiveMessageHandler = fn
}

func (t *cloudTunnel) RegistClientCloseHandler(fn ClientCloseHandleFunc) {
	t.notifyClientClosed = fn
}

func (t *cloudTunnel) RegistAfterConnectHook(fn AfterConnectHook) {
	t.afterConnectHook = fn
}

func (t *cloudTunnel) handleReceiveMessage(client *WSClient) {
	if client == nil {
		return
	}

	klog.Infof("wsclient %s start read message", client.Name)
	for {
		msg, err := client.ReadMessage()
		if err != nil {
			klog.Errorf("wsclient %s read msg error, err:%s", client.Name, err.Error())
			break
		}
		t.receiveMessageHandler(client.Name, msg)
	}
}

func (t *cloudTunnel) connect(cr *config.ClusterRegistry, conn *websocket.Conn) {
	wsclient := NewWSClient(cr.Name, conn)
	_, ok := t.clients.LoadOrStore(cr.Name, wsclient)
	if ok {
		klog.Infof("cluster %s is already connected", cr.Name)
		if err := wsclient.Close(); err != nil {
			klog.Errorf("close websocket connection failed: %s", err.Error())
		}
		return
	}

	klog.Infof("cluster %s is connected", cr.Name)
	t.afterConnectHook(cr)
	t.handleReceiveMessage(wsclient)

	// notify client closed.
	klog.Infof("cluster %s is disconnected", cr.Name)
	cr.Time = time.Now().Unix()
	go t.notifyClientClosed(cr)

	// close websocket.
	wsclient.Close()
	t.clients.Delete(cr.Name)
}

func (t *cloudTunnel) accessHandler(w http.ResponseWriter, r *http.Request) {
	cluster := mux.Vars(r)[accessURIParam]

	// get cluster listen addr from header.
	// TODO if listen addr is duplicated, refuse to connect.
	listenAddr := r.Header.Get(config.ClusterConnectHeaderListenAddr)
	if listenAddr == "" {
		klog.V(1).Infof("cluster %s listenAddr is not specified, should set in header", cluster)
		http.Error(w, "listenAddr is not specified, should set in header", http.StatusBadRequest)
		return
	}
	// get name of the child
	name := r.Header.Get(config.ClusterConnectHeaderUserDefineName)
	if name == "" {
		klog.V(1).Infof("cluster %s user-define name is not specified, should set in header", cluster)
		http.Error(w, "user-define name is not specified, should set in header", http.StatusBadRequest)
		return
	}

	_, ok := t.clients.Load(cluster)
	if ok {
		klog.V(1).Infof("cluster %s is already connected", cluster)
		http.Error(w, "already build connection", http.StatusForbidden)
		return
	}

	cr := config.ClusterRegistry{
		Name:           cluster,
		UserDefineName: name,
		Listen:         listenAddr,
		Time:           time.Now().Unix(),
	}

	if !t.clusterNameCheck(&cr) {
		klog.V(1).Infof("cluster %s has been registered", cluster)
		http.Error(w, "cluster name has been registered", http.StatusForbidden)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		klog.Errorf("connect to cluster %s failed: %s", cluster, err.Error())
		http.Error(w, "fail to upgrade to websocket", http.StatusInternalServerError)
		return
	}

	go t.connect(&cr, conn)
}

func (t *cloudTunnel) Stop() error {
	// TODO gradeful stop cloudtunnel.
	return nil
}

func (t *cloudTunnel) Start() error {
	router := mux.NewRouter()
	uri := fmt.Sprintf(accessURIPattern, accessURIParam)
	router.HandleFunc(uri, t.accessHandler)

	// TODO set request timeout.
	s := http.Server{
		Addr:    fmt.Sprintf("%s", t.address),
		Handler: router,
	}

	go func() {
		if err := s.ListenAndServe(); err != nil {
			klog.Fatalf("fail to start cloudtunnel: %s", err.Error())
		}
	}()

	return nil
}

func defaultClusterNameChecker(cr *config.ClusterRegistry) bool {
	return true
}

func defaultAfterConnectHook(cr *config.ClusterRegistry) {}
