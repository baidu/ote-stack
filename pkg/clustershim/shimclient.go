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
	"github.com/baidu/ote-stack/pkg/clustermessage"
	"github.com/baidu/ote-stack/pkg/clustershim/handler"
	"github.com/baidu/ote-stack/pkg/config"
	"github.com/baidu/ote-stack/pkg/k8sclient"
	"github.com/baidu/ote-stack/pkg/tunnel"
)

const (
	shimRespChanLen = 100
)

// ShimServiceClient is the client interface to a cluster shim.
type ShimServiceClient interface {
	Do(in *clustermessage.ClusterMessage) (*clustermessage.ClusterMessage, error)
	ReturnChan() <-chan *clustermessage.ClusterMessage
}

type localShimClient struct {
	handlers map[string]handler.Handler
}

type remoteShimClient struct {
	client   *tunnel.WSClient
	respChan chan *clustermessage.ClusterMessage
}

// ShimHandler is a handler map of a shim server.
// The key is Destination field in ShimRequest,
// and the value is the corresponding handler.
type ShimHandler map[string]handler.Handler

// NewlocalShimClient returns a local shim client with default handler.
func NewlocalShimClient(c *config.ClusterControllerConfig) ShimServiceClient {
	k8sClient, err := k8sclient.NewK8sClient(k8sclient.NewK8sOption(c.KubeConfig, 0))
	if err != nil {
		klog.Errorf("failed to create k8s client: %v", err)
		return nil
	}

	local := &localShimClient{
		handlers: make(map[string]handler.Handler),
	}

	local.handlers[otev1.ClusterControllerDestAPI] = handler.NewK8sHandler(k8sClient)
	local.handlers[otev1.ClusterControllerDestHelm] = handler.NewHTTPProxyHandler(c.HelmTillerAddr)
	return local
}

// NewlocalShimClientWithHandler returns a local shim client with given handlers.
func NewlocalShimClientWithHandler(handlers ShimHandler) ShimServiceClient {
	return &localShimClient{
		handlers: handlers,
	}
}

func (s *localShimClient) Do(in *clustermessage.ClusterMessage) (*clustermessage.ClusterMessage, error) {
	switch in.Head.Command {
	case clustermessage.CommandType_ControlReq:
		return s.DoControlRequest(in)
	case clustermessage.CommandType_ControlMultiReq:
		return nil, s.DoControlMultiRequest(in)
	default:
		return nil, fmt.Errorf("command %s is not supported by ShimClient", in.Head.Command.String())
	}
}

func (s *localShimClient) DoControlRequest(in *clustermessage.ClusterMessage) (*clustermessage.ClusterMessage, error) {
	head := proto.Clone(in.Head).(*clustermessage.MessageHead)
	head.Command = clustermessage.CommandType_ControlResp

	controllerTask := handler.GetControllerTaskFromClusterMessage(in)
	if controllerTask == nil {
		resp := handler.ControlTaskResponse(http.StatusNotFound, "")
		return handler.Response(resp, head), fmt.Errorf("ControllerTask Not Found")
	}

	h, exist := s.handlers[controllerTask.Destination]
	if exist {
		resp, err := h.Do(in)
		if resp != nil {
			resp.Head.Command = clustermessage.CommandType_ControlResp
		}
		return resp, err
	}

	resp := handler.ControlTaskResponse(http.StatusNotFound, "")
	return handler.Response(resp, head), fmt.Errorf("no handler for %s", controllerTask.Destination)
}

func (s *localShimClient) DoControlMultiRequest(in *clustermessage.ClusterMessage) error {
	controlMultiTask := handler.GetControlMultiTaskFromClusterMessage(in)
	if controlMultiTask == nil {
		return fmt.Errorf("ControlMultiTask Not Found")
	}

	h, exist := s.handlers[controlMultiTask.Destination]
	if exist {
		_, err := h.Do(in)
		return err
	}

	return fmt.Errorf("no handler for %s", controlMultiTask.Destination)
}

func (s *localShimClient) ReturnChan() <-chan *clustermessage.ClusterMessage {
	return nil
}

// NewRemoteShimClient returns a remote shim client which is connecting to addr.
func NewRemoteShimClient(shimClientName, addr string) ShimServiceClient {
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
		respChan: make(chan *clustermessage.ClusterMessage, shimRespChanLen),
	}
	go ret.handleReceiveMessage()
	return ret
}

func (s *remoteShimClient) Do(in *clustermessage.ClusterMessage) (*clustermessage.ClusterMessage, error) {
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

func (s *remoteShimClient) ReturnChan() <-chan *clustermessage.ClusterMessage {
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

		// unserialize msg to ClusterMessage
		resp := &clustermessage.ClusterMessage{}
		err = proto.Unmarshal(msg, resp)
		if err != nil {
			klog.Errorf("unmarshal shim response failed: %v", err)
			continue
		}
		s.respChan <- resp
	}
}
