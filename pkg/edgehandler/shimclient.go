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

package edgehandler

import (
	"context"
	"fmt"
	"net"
	"time"

	"google.golang.org/grpc"
	"k8s.io/klog"

	otev1 "github.com/baidu/ote-stack/pkg/apis/ote/v1"
	pb "github.com/baidu/ote-stack/pkg/clustershim/apis/v1"
	"github.com/baidu/ote-stack/pkg/clustershim/handler"
	"github.com/baidu/ote-stack/pkg/config"
)

type shimServiceClient interface {
	Do(in *pb.ShimRequest) (*pb.ShimResponse, error)
}

type localShimClient struct {
	handlers map[string]handler.Handler
}

type remoteShimClient struct {
	client pb.ClusterShimServiceClient
}

func newLocalShimClient(c *config.ClusterControllerConfig) shimServiceClient {
	local := &localShimClient{
		handlers: make(map[string]handler.Handler),
	}
	local.handlers[otev1.CLUSTER_CONTROLLER_DEST_API] = handler.NewK8sHandler(c.K8sClient)
	local.handlers[otev1.CLUSTER_CONTROLLER_DEST_HELM] = handler.NewHTTPProxyHandler(c.HelmTillerAddr)
	return local
}

func (s *localShimClient) Do(in *pb.ShimRequest) (*pb.ShimResponse, error) {
	h, exist := s.handlers[in.Destination]
	if exist {
		return h.Do(in)
	}

	return nil, fmt.Errorf("no handler for %s", in.Destination)
}

func unixConnect(addr string, t time.Duration) (net.Conn, error) {
	unixAddr, err := net.ResolveUnixAddr("unix", addr)
	conn, err := net.DialUnix("unix", nil, unixAddr)
	return conn, err
}

func newRemoteShimClient(addr string) shimServiceClient {
	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithDialer(unixConnect))
	if err != nil {
		klog.Errorf("dial error: %v\n", err)
		return nil
	}

	klog.Infof("dial to %s", addr)
	return &remoteShimClient{
		client: pb.NewClusterShimServiceClient(conn),
	}
}

func (s *remoteShimClient) Do(in *pb.ShimRequest) (*pb.ShimResponse, error) {
	return s.client.Do(context.Background(), in)
}
