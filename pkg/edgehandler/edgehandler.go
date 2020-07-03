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

// Package edgehandler maintenances the websocket connection with cloud server
// and process the receive messages.
package edgehandler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/golang/protobuf/proto"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/clustermessage"
	clusterrouter "github.com/baidu/ote-stack/pkg/clusterrouter"
	"github.com/baidu/ote-stack/pkg/clusterselector"
	"github.com/baidu/ote-stack/pkg/clustershim"
	"github.com/baidu/ote-stack/pkg/config"
	"github.com/baidu/ote-stack/pkg/tunnel"
)

var (
	subtreeReportDuration       = 1 * time.Second
	sendToParentTimeout         = 1 * time.Second
	sendToParentChan            = make(chan []byte, 10000)
	sendToClusterHandlerTimeout = 1 * time.Second
)

// EdgeHandler is edgehandler interface that process messages from tunnel and transmit to shim.
type EdgeHandler interface {
	// Start will start edgehandler.
	Start() error
}

// edgeHandler processes message from tunnel and transmit to shim.
type edgeHandler struct {
	conf              *config.ClusterControllerConfig
	edgeTunnel        tunnel.EdgeTunnel
	shimClient        clustershim.ShimServiceClient
	stopReportSubtree chan struct{}
	rootClusterEnable bool
}

// NewEdgeHandler returns a edgeHandler object.
func NewEdgeHandler(c *config.ClusterControllerConfig) EdgeHandler {
	return &edgeHandler{
		conf:              c,
		stopReportSubtree: make(chan struct{}, 1),
		rootClusterEnable: false,
	}
}

func (e *edgeHandler) valid() error {
	if e.conf.ClusterUserDefineName == "" {
		return fmt.Errorf("cluster name is empty")
	}
	if e.conf.K8sClient == nil && !e.isRemoteShim() {
		return fmt.Errorf("k8s client is unavailable or remoteshim not set")
	}
	if e.conf.ParentCluster == "" && !e.isRoot() {
		return fmt.Errorf("parent cluster is empty")
	}
	if e.conf.ParentCluster != "" && e.isRoot() {
		return fmt.Errorf("root cc should not have parent cluster")
	}

	if e.isRoot() && e.isRemoteShim() {
		// if it is root cc, and connectes to shim, it should can be a single root cluster.
		e.rootClusterEnable = true
	}

	return nil
}

func (e *edgeHandler) isRoot() bool {
	return config.IsRoot(e.conf.ClusterUserDefineName)
}

func (e *edgeHandler) isRemoteShim() bool {
	return e.conf.RemoteShimAddr != ""
}

func (e *edgeHandler) Start() error {
	if err := e.valid(); err != nil {
		return err
	}

	if e.isRemoteShim() {
		klog.Infof("init remote shim client")
		e.shimClient = clustershim.NewRemoteShimClient(e.conf.ClusterName, e.conf.RemoteShimAddr)
	} else if !e.isRoot() {
		klog.Infof("init local shim client")
		e.shimClient = clustershim.NewlocalShimClient(e.conf)
	}

	if e.shimClient == nil && (e.rootClusterEnable || !e.isRoot()) {
		return fmt.Errorf("fail to init shim client")
	}

	if !e.isRoot() {
		e.edgeTunnel = tunnel.NewEdgeTunnel(e.conf)
		e.edgeTunnel.RegistReceiveMessageHandler(e.receiveMessageFromTunnel)
		e.edgeTunnel.RegistAfterConnectToHook(e.afterConnect)
		e.edgeTunnel.RegistAfterDisconnectHook(e.afterDisconnect)
		if err := e.edgeTunnel.Start(); err != nil {
			return err
		}

		// send the msg from cluster handler and shim.
		go e.sendMessageToParent()

		// handle the msg from cluster handler.
		go e.sendMessageToTunnel()
	}

	// single cluster's rootcc should handle msg from rootcm.
	if e.rootClusterEnable {
		go func() {
			for {
				msg := <-e.conf.RootClusterToEdgeChan
				e.handleMessage(msg)
			}
		}()
	}

	go e.handleRespFromShimClient()

	return nil
}

// sendMessageToParent has multi tasks for reading the message from sendToParentChan,
// and sending the message to parent cluster.
func (e *edgeHandler) sendMessageToParent() {
	for {
		data := <-sendToParentChan
		e.edgeTunnel.Send(data)
	}
}

func (e *edgeHandler) sendMessageToTunnel() {
	for {
		msg := <-e.conf.ClusterToEdgeChan

		e.sendToParent(&msg)
	}
}

func (e *edgeHandler) receiveMessageFromTunnel(client string, data []byte) (ret error) {
	ret = nil
	msg := &clustermessage.ClusterMessage{}
	err := proto.Unmarshal(data, msg)
	if err != nil {
		ret = fmt.Errorf("can not deserialize message, error: %s", err.Error())
		klog.Error(ret)
		return
	}

	e.conf.EdgeToClusterChan <- *msg

	selector := clusterselector.NewSelector(msg.Head.ClusterSelector)
	if selector.Has(e.conf.ClusterName) {
		e.handleMessage(msg)
	}

	return
}

func responseErrorStatus(err error) []byte {
	resp := &clustermessage.ControllerTaskResponse{
		Timestamp:  time.Now().Unix(),
		Body:       []byte(err.Error()),
		StatusCode: http.StatusInternalServerError,
	}
	data, err := proto.Marshal(resp)
	if err != nil {
		klog.Errorf("marshal controller task resp failed: %v", err)
		return nil
	}
	return data
}

func (e *edgeHandler) handleMessage(msg *clustermessage.ClusterMessage) error {
	switch msg.Head.Command {
	case clustermessage.CommandType_ControlReq:
		klog.V(1).Infof("dispatch message %v to shim", msg.Head.MessageID)
		resp, err := e.shimClient.Do(msg)
		if resp != nil {
			// sync return
			if err != nil {
				resp.Body = responseErrorStatus(err)
				klog.Errorf("handleTask error: %s", err.Error())
			}

			resp.Head.ClusterName = e.conf.ClusterName
			// send to cloudtunnel.
			err = e.sendToParent(resp)
		} else {
			if err != nil {
				klog.Errorf("handleTask error: %v", err)
			}
		}
		return err
	case clustermessage.CommandType_ControlMultiReq:
		klog.V(3).Infof("dispatch ControlMultiReq message to shim")
		_, err := e.shimClient.Do(msg)
		if err != nil {
			klog.Errorf("handleTask error: %s", err.Error())
		}
		return err
	default:
		klog.Errorf("command %s is not supported by edge handler", msg.Head.Command.String())
		return nil
	}
}

func (e *edgeHandler) handleRespFromShimClient() {
	// async return
	if e.shimClient == nil || e.shimClient.ReturnChan() == nil {
		klog.Warningf("shim client or return chan is nil, cannot handle resp")
		return
	}
	respChan := e.shimClient.ReturnChan()
	if respChan == nil {
		klog.Warningf("async return channel from shim client is nil")
		return
	}

	for {
		resp := <-respChan

		resp.Head.ClusterName = e.conf.ClusterName

		if e.rootClusterEnable {
			// send to clusterhandler
			select {
			case e.conf.RootEdgeToClusterChan <- resp:
				klog.V(5).Info("send report msg to cluster handler success")
			case <-time.After(sendToClusterHandlerTimeout):
				klog.V(5).Info("send report msg to cluster handler timeout")
			}
		} else {
			// send to cloudtunnel.
			e.sendToParent(resp)
		}
	}
	klog.Warningf("async return channel from shim client closed")
}

func (e *edgeHandler) afterConnect() {
	// start subtree report goroutine
	go e.reportSubTreeTimer()
}

func (e *edgeHandler) afterDisconnect() {
	// stop subtree report goroutine
	e.stopReportSubtree <- struct{}{}
}

func (e *edgeHandler) reportSubTreeTimer() {
	klog.Info("start reporting subtree")

	// call report once and start timer
	e.reportSubTree()

	ticker := time.NewTicker(subtreeReportDuration)
	for {
		select {
		case <-e.stopReportSubtree:
			klog.Info("stop reporting subtree")
			return
		case <-ticker.C:
			e.reportSubTree()
		}
	}
}

func (e *edgeHandler) reportSubTree() {
	msg := clusterrouter.Router().SubTreeMessage()
	if msg == nil {
		return
	}
	msg.Head.ClusterName = e.conf.ClusterName
	e.sendToParent(msg)
}

func (e *edgeHandler) sendToParent(msg *clustermessage.ClusterMessage) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		klog.Errorf("marshal cluster message error: %s", err.Error())
		return err
	}

	select {
	case sendToParentChan <- data:
		klog.V(5).Info("send msg to parent success")
	case <-time.After(sendToParentTimeout):
		klog.V(5).Info("send msg to parent timeout")
	}

	return nil
}
