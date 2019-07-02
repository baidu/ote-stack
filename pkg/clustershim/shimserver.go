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
	"net"
	"net/http"

	"google.golang.org/grpc"
	"k8s.io/klog"

	pb "github.com/baidu/ote-stack/pkg/clustershim/apis/v1"
	"github.com/baidu/ote-stack/pkg/clustershim/handler"
)

// ShimServer handles requests and transmits to corresponding shim handler.
type ShimServer struct {
	handlers map[string]handler.Handler
	server   *grpc.Server
}

// NewShimServer creates a new shimServer.
func NewShimServer() *ShimServer {
	return &ShimServer{
		handlers: make(map[string]handler.Handler),
		server:   grpc.NewServer(),
	}
}

// RegisterHandler registers shim handler.
func (s *ShimServer) RegisterHandler(name string, h handler.Handler) {
	s.handlers[name] = h
}

// Do handles the requests and transmits to corresponding server.
func (s *ShimServer) Do(ctx context.Context, in *pb.ShimRequest) (*pb.ShimResponse, error) {
	klog.V(1).Infof("Received request for %v", in.Destination)

	h, exist := s.handlers[in.Destination]
	if exist {
		resp, err := h.Do(in)
		if err != nil {
			klog.Errorf("handle request error: %v", err)
		}
		return resp, err
	}

	klog.Infof("no handler for %v", in.Destination)
	return handler.Response(http.StatusNotFound, ""), fmt.Errorf("Not Found")
}

// Serve starts a grpc server over unix socket.
// TODO: support ip address connection.
func (s *ShimServer) Serve(sockFile string) error {
	addr, err := net.ResolveUnixAddr("unix", sockFile)
	if err != nil {
		return err
	}

	listener, err := net.ListenUnix("unix", addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	klog.Infof("listen %s", sockFile)

	pb.RegisterClusterShimServiceServer(s.server, s)
	if err := s.server.Serve(listener); err != nil {
		return err
	}

	return nil
}

// Close gracefully stops shim server.
func (s *ShimServer) Close() {
	s.server.GracefulStop()
}
