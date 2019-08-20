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
	"github.com/golang/protobuf/proto"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/clustermessage"
	pb "github.com/baidu/ote-stack/pkg/clustershim/apis/v1"
)

func pb2SerializedControllerTaskResp(
	in *pb.ShimResponse) (*pb.MessageHead, []byte) {
	resp := &clustermessage.ControllerTaskResponse{
		Timestamp:  in.Timestamp,
		StatusCode: in.StatusCode,
		Body:       []byte(in.Body),
	}
	data, err := proto.Marshal(resp)
	if err != nil {
		klog.Errorf("transfer shim resp to controller task resp failed: %v", err)
		return nil, nil
	}
	return in.Head, data
}

func controllerTask2Pb(
	msg *clustermessage.ClusterMessage, task *clustermessage.ControllerTask) *pb.ShimRequest {
	return &pb.ShimRequest{
		ParentClusterName: msg.Head.ParentClusterName,
		Destination:       task.Destination,
		Method:            task.Method,
		URL:               task.URI,
		Body:              string(task.Body),
		Head: &pb.MessageHead{
			MessageID:         msg.Head.MessageID,
			ParentClusterName: msg.Head.ParentClusterName,
		},
	}
}
