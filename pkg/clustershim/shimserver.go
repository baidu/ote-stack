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

// Package clustershim implements a grpc server for handling clustercontroller requests.
package clustershim

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"k8s.io/klog"
	
	"github.com/baidu/ote-stack/pkg/clustermessage"
	"github.com/baidu/ote-stack/pkg/clustershim/handler"
	"github.com/baidu/ote-stack/pkg/tunnel"
)

const (
	clusterNameParam                   = "cc_name"
	shimServerPathForClusterController = "clustercontroller"
)

var (
	upgrader = websocket.Upgrader{}
)

// ShimServer handles requests and transmits to corresponding shim handler.
type ShimServer struct {
	handlers    map[string]handler.Handler
	server      *http.Server
	ccclient    *tunnel.WSClient
	clientMutex *sync.RWMutex
	clusterName string
}

// NewShimServer creates a new shimServer.
func NewShimServer() *ShimServer {
	return &ShimServer{
		handlers:    make(map[string]handler.Handler),
		clientMutex: &sync.RWMutex{},
	}
}

// RegisterHandler registers shim handler.
func (s *ShimServer) RegisterHandler(name string, h handler.Handler) {
	s.handlers[name] = h
}

// Do handles the requests and transmits to corresponding server.
func (s *ShimServer) Do(in *clustermessage.ClusterMessage) (*clustermessage.ClusterMessage, error) {
	switch in.Head.Command {
	case clustermessage.CommandType_ControlReq:
		return s.DoControlRequest(in)
	default:
		return nil, fmt.Errorf("command %s is not supported by ShimServer", in.Head.Command.String())
	}
}

func (s *ShimServer) DoControlRequest(in *clustermessage.ClusterMessage) (*clustermessage.ClusterMessage, error) {
	head := proto.Clone(in.Head).(*clustermessage.MessageHead)
	head.Command = clustermessage.CommandType_ControlResp

	controllerTask := handler.GetControllerTaskFromClusterMessage(in)
	if controllerTask == nil {
		resp := handler.ControlTaskResponse(http.StatusNotFound, "")
		return handler.Response(resp, head), fmt.Errorf("Controllertask Not Found")
	}
	klog.V(1).Infof("Received request for %v", controllerTask.Destination)

	h, exist := s.handlers[controllerTask.Destination]
	if exist {
		resp, err := h.Do(in)

		if err != nil {
			klog.Errorf("handle request error: %v", err)
		}
		if resp != nil {
			resp.Head.Command = clustermessage.CommandType_ControlResp
		}
		return resp, err
	}

	klog.Infof("no handler for %v", controllerTask.Destination)
	resp := handler.ControlTaskResponse(http.StatusNotFound, "")
	return handler.Response(resp, head), fmt.Errorf("Not Found")
}

func (s *ShimServer) do(w http.ResponseWriter, r *http.Request) {
	if s.ccclient != nil {
		msg := "there is already a cluster controller connected"
		klog.Errorf(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	s.clusterName = mux.Vars(r)[clusterNameParam]

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		klog.Errorf("connect to cluster controller %s failed: %s", s.clusterName, err.Error())
		http.Error(w, "fail to upgrade to websocket", http.StatusInternalServerError)
		return
	}

	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()
	s.ccclient = tunnel.NewWSClient(s.clusterName, conn)
	// connected is a block function, must call it in goroutine to release http resources
	go s.connected()
}

func (s *ShimServer) connected() {
	klog.Infof("cluster controller connected")
	// readMessage is a block function
	s.readMessage()

	klog.Infof("cluster controller %s is disconnected", s.clusterName)
	// clear client to wait for next
	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()
	s.ccclient.Close()
	s.ccclient = nil
}

func (s *ShimServer) readMessage() {
	for {
		msg, err := s.ccclient.ReadMessage()
		if err != nil {
			klog.Errorf("wsclient %s read msg error, err:%s", s.ccclient.Name, err.Error())
			break
		}
		s.handleReadMessage(msg)
	}
}

func (s *ShimServer) handleReadMessage(msg []byte) {
	in := clustermessage.ClusterMessage{}
	err := proto.Unmarshal(msg, &in)
	if err != nil {
		klog.Errorf("unmarshal shim request failed: %v", err)
		return
	}

	resp, err := s.Do(&in)
	if resp == nil {
		klog.Errorf("execute shim request failed!")
		return
	}
	if err != nil {
		klog.Errorf("execute shim request failed: %v", err)
	}
	respMsg, err := proto.Marshal(resp)
	if err != nil {
		klog.Errorf("marshal shim response failed: %v", err)
		return
	}
	s.clientMutex.RLock()
	defer s.clientMutex.RUnlock()
	s.ccclient.WriteMessage(respMsg)
}

// Serve starts a grpc server over unix socket.
// TODO: support ip address connection.
func (s *ShimServer) Serve(addr string) error {
	router := mux.NewRouter()
	router.HandleFunc(fmt.Sprintf("/%s/{%s}",
		shimServerPathForClusterController, clusterNameParam), s.do)

	s.server = &http.Server{
		Addr:         addr,
		Handler:      router,
		WriteTimeout: tunnel.WriteTimeout,
		ReadTimeout:  tunnel.ReadTimeout,
		IdleTimeout:  tunnel.IdleTimeout,
	}

	klog.Infof("listen on %s", addr)
	if err := s.server.ListenAndServe(); err != nil {
		klog.Fatalf("fail to start cloudtunnel: %s", err.Error())
	}

	return nil
}

// Close gracefully stops shim server.
func (s *ShimServer) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), tunnel.StopTimeout)
	defer cancel()
	s.server.Shutdown(ctx)
}
