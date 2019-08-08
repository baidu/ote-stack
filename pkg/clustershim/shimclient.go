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

package clustershim

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	"k8s.io/klog"

	otev1 "github.com/baidu/ote-stack/pkg/apis/ote/v1"
	pb "github.com/baidu/ote-stack/pkg/clustershim/apis/v1"
	"github.com/baidu/ote-stack/pkg/clustershim/handler"
	"github.com/baidu/ote-stack/pkg/config"
	"github.com/baidu/ote-stack/pkg/tunnel"
)

const (
	shimRespChanLen = 100
	shimClientName  = "clustercontroller"
)

type ShimServiceClient interface {
	Do(in *pb.ShimRequest) (*pb.ShimResponse, error)
	ReturnChan() <-chan *pb.ShimResponse
}

type localShimClient struct {
	handlers map[string]handler.Handler
}

type remoteShimClient struct {
	client   *tunnel.WSClient
	respChan chan *pb.ShimResponse
}

type ShimHandler map[string]handler.Handler

func NewlocalShimClient(c *config.ClusterControllerConfig) ShimServiceClient {
	local := &localShimClient{
		handlers: make(map[string]handler.Handler),
	}
	local.handlers[otev1.ClusterControllerDestAPI] = handler.NewK8sHandler(c.K8sClient)
	local.handlers[otev1.ClusterControllerDestHelm] = handler.NewHTTPProxyHandler(c.HelmTillerAddr)
	return local
}

func NewlocalShimClientWithHandler(handlers ShimHandler) ShimServiceClient {
	return &localShimClient{
		handlers: handlers,
	}
}

func (s *localShimClient) Do(in *pb.ShimRequest) (*pb.ShimResponse, error) {
	h, exist := s.handlers[in.Destination]
	if exist {
		return h.Do(in)
	}

	return &pb.ShimResponse{}, fmt.Errorf("no handler for %s", in.Destination)
}

func (s *localShimClient) ReturnChan() <-chan *pb.ShimResponse {
	return nil
}

func NewRemoteShimClient(addr string) ShimServiceClient {
	u := url.URL{
		Scheme: "ws",
		Host:   addr,
		Path:   fmt.Sprintf("/%s/%s", shimServerPathForClusterController, shimClientName),
	}
	header := http.Header{}
	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), header)
	if err != nil {
		if resp != nil {
			klog.Errorf("failed to connect to remote shim, code=%v", resp.StatusCode)
		}
		klog.Errorf("failed to connect to remote shim: %v", err)
		return nil
	}
	ret := &remoteShimClient{
		client:   tunnel.NewWSClient(shimClientName, conn),
		respChan: make(chan *pb.ShimResponse, shimRespChanLen),
	}
	go ret.handleReceiveMessage()
	return ret
}

func (s *remoteShimClient) Do(in *pb.ShimRequest) (*pb.ShimResponse, error) {
	// serialized req and send to server
	reqMsg, err := proto.Marshal(in)
	if err != nil {
		msg := fmt.Sprintf("marshal shim request failed: %v", err)
		klog.Error(msg)
		return nil, fmt.Errorf(msg)
	}
	go s.client.WriteMessage(reqMsg)
	return nil, nil
}

func (s *remoteShimClient) ReturnChan() <-chan *pb.ShimResponse {
	return s.respChan
}

func (s *remoteShimClient) handleReceiveMessage() {
	klog.V(1).Infof("start handle receive message")
	for {
		msg, err := s.client.ReadMessage()
		if err != nil {
			klog.Errorf("read msg failed: %s", err.Error())
			break
		}

		// unserialize msg to ShimResponse
		resp := &pb.ShimResponse{}
		err = proto.Unmarshal(msg, resp)
		if err != nil {
			klog.Errorf("unmarshal shim request failed: %v", err)
			continue
		}
		s.respChan <- resp
	}
}
