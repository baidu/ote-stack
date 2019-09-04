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
	"context"
	"fmt"
	"math/rand"
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

	// uri for ote controller manager
	controllerURI = "/controller"
)

var upgrader = websocket.Upgrader{}

// ControllerManagerMsgHandleFunc is a function handle msg from controller manager,
// string is remote address of the controller manager,
// and []byte is the msg.
type ControllerManagerMsgHandleFunc func(string, []byte) error

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
	// SendToControllerManager sends msg to anyone of controller manager.
	SendToControllerManager([]byte) error
	// RegistCheckNameValidFunc registers ClusterNameChecker.
	RegistCheckNameValidFunc(fn ClusterNameChecker)
	// RegistAfterConnectHook registers AfterConnectHook.
	RegistAfterConnectHook(fn AfterConnectHook)
	// RegistReturnMessageFunc registers TunnelReadMessageFunc.
	RegistReturnMessageFunc(fn TunnelReadMessageFunc)
	// RegistClientCloseHandler registers ClientCloseHandleFunc.
	RegistClientCloseHandler(fn ClientCloseHandleFunc)
	// RegistControllerManagerMsgHandler regists ControllerManagerMsgHandleFunc.
	RegistControllerManagerMsgHandler(fn ControllerManagerMsgHandleFunc)
}

// cloudTunnel handles all communications with edgetunnel.
type cloudTunnel struct {
	clients               sync.Map
	address               string
	clusterNameCheck      ClusterNameChecker
	receiveMessageHandler TunnelReadMessageFunc
	notifyClientClosed    ClientCloseHandleFunc
	afterConnectHook      AfterConnectHook
	server                *http.Server
	controllers           sync.Map // remoteAddr -> wsclient
	controllersKey        []string
	controlMsgHandler     ControllerManagerMsgHandleFunc
}

// NewCloudTunnel returns a new cloudTunnel object.
func NewCloudTunnel(address string) CloudTunnel {
	tunnel := &cloudTunnel{
		address:            address,
		clusterNameCheck:   defaultClusterNameChecker,
		notifyClientClosed: func(*config.ClusterRegistry) { return },
		afterConnectHook:   defaultAfterConnectHook,
		controlMsgHandler:  defaultControlMsgHandler,
		controllersKey:     make([]string, 0),
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

func (t *cloudTunnel) SendToControllerManager(msg []byte) error {
	// select a controller and send the msg
	client := findAController(t.controllers, t.controllersKey)
	if client == nil {
		return fmt.Errorf("cannot find a controller to send msg to")
	}
	return client.WriteMessage(msg)
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

func (t *cloudTunnel) RegistControllerManagerMsgHandler(fn ControllerManagerMsgHandleFunc) {
	t.controlMsgHandler = fn
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

// handler for child cluster controller
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

func (t *cloudTunnel) controllerHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		klog.Errorf("connect to controller %s failed: %s", r.RemoteAddr, err.Error())
		http.Error(w, "fail to upgrade to websocket", http.StatusInternalServerError)
		return
	}
	wsclient := NewWSClient(r.RemoteAddr, conn)
	_, ok := t.controllers.LoadOrStore(r.RemoteAddr, wsclient)
	if ok {
		klog.Infof("controller %s is already connected", r.RemoteAddr)
		if err := wsclient.Close(); err != nil {
			klog.Errorf("close websocket connection failed: %s", err.Error())
		}
		return
	}
	klog.Infof("controller %s is connected", r.RemoteAddr)
	t.controllersKey = append(t.controllersKey, r.RemoteAddr)
	// root cluster controller get msg from controllers and publish to clusters
	go t.handleControlMsg(wsclient)
}

func (t *cloudTunnel) handleControlMsg(client *WSClient) {
	if client == nil {
		return
	}

	klog.Infof("wsclient %s start read control message", client.Name)
	for {
		msg, err := client.ReadMessage()
		if err != nil {
			klog.Errorf("wsclient %s read msg error, err:%s", client.Name, err.Error())
			break
		}
		t.controlMsgHandler(client.Name, msg)
	}

	klog.Infof("cluster %s is disconnected", client.Name)

	// close websocket.
	t.controllers.Delete(client.Name)
	t.controllersKey = removeFromSliceByValue(t.controllersKey, client.Name)
	client.Close()
}

func (t *cloudTunnel) Stop() error {
	// gradeful stop cloudtunnel.
	ctx, cancel := context.WithTimeout(context.Background(), StopTimeout)
	defer cancel()
	return t.server.Shutdown(ctx)
}

func (t *cloudTunnel) Start() error {
	router := mux.NewRouter()
	uri := fmt.Sprintf(accessURIPattern, accessURIParam)
	router.HandleFunc(uri, t.accessHandler)
	// add handler for ote controller manager
	router.HandleFunc(controllerURI, t.controllerHandler)

	t.server = &http.Server{
		Addr:         fmt.Sprintf("%s", t.address),
		Handler:      router,
		WriteTimeout: WriteTimeout,
		ReadTimeout:  ReadTimeout,
		IdleTimeout:  IdleTimeout,
	}

	go func() {
		if err := t.server.ListenAndServe(); err != nil {
			klog.Fatalf("fail to start cloudtunnel: %s", err.Error())
		}
	}()

	return nil
}

func defaultClusterNameChecker(cr *config.ClusterRegistry) bool {
	return true
}

func defaultAfterConnectHook(cr *config.ClusterRegistry) {}

func defaultControlMsgHandler(remote string, msg []byte) error {
	return nil
}

func removeFromSliceByValue(slice []string, s string) []string {
	n := -1
	for i, v := range slice {
		if v == s {
			n = i
			break
		}
	}
	if n < 0 {
		klog.Warningf("not found %s in slice %v", s, slice)
		return slice
	}
	return append(slice[:n], slice[n+1:]...)
}

func findAController(clientMap sync.Map, clientKey []string) *WSClient {
	if clientKey == nil || len(clientKey) == 0 {
		klog.Errorf("client map and key are empty to find a controller")
		return nil
	}

	clientName := clientKey[rand.Intn(len(clientKey))]
	client, ok := clientMap.Load(clientName)
	if ok {
		return client.(*WSClient)
	}

	klog.Warningf("client %s is in client key but has no client(%v)", clientName, clientMap)
	return findAController(clientMap, removeFromSliceByValue(clientKey, clientName))
}
