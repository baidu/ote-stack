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

package controllermanager

import (
	"fmt"

	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/clustermessage"
)

// UpstreamProcessor processes msg from root cluster controller.
type UpstreamProcessor struct {
	ctx *K8sContext
}

// NewUpstreamProcessor new a UpstreamProcessor with k8s context.
func NewUpstreamProcessor(ctx *K8sContext) *UpstreamProcessor {
	return &UpstreamProcessor{ctx: ctx}
}

// HandleReceivedMessage processes msg from root cluster controller.
// This function should be registed to controller tunnel.
func (u *UpstreamProcessor) HandleReceivedMessage(client string, data []byte) (ret error) {
	// get ClusterMessage from data
	msg := &clustermessage.ClusterMessage{}
	err := msg.Deserialize(data)
	if err != nil {
		ret = fmt.Errorf("handleReceivedMessage failed %v", err)
		klog.Errorf("%v", ret)
		return
	}

	if msg.Head == nil {
		ret = fmt.Errorf("handleReceivedMessage failed: message head is nil")
		klog.Error(ret)
		return
	}

	// TODO add other command cases
	switch msg.Head.Command {
	case clustermessage.CommandType_EdgeReport:
		ret = processEdgeReport(msg)
	default:
		ret = fmt.Errorf("handleReceivedMessage failed: %s command not supported", msg.Head.Command.String())
		klog.Error(ret)
	}
	return
}

// TODO process edge report
func processEdgeReport(msg *clustermessage.ClusterMessage) error {
	return nil
}
