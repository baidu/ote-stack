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

// Package handler provides the ability to interact with k8s or third-party services.
package handler

import (
	"time"

	"github.com/golang/protobuf/proto"
	"k8s.io/klog"
	
	"github.com/baidu/ote-stack/pkg/clustermessage"
)

// Handler is shim handler interface that contains
// the methods required to interact to remote server.
type Handler interface {
	// Do handle the request and transmit to corresponding server.
	Do(*clustermessage.ClusterMessage) (*clustermessage.ClusterMessage, error)
}

// Response packages the body message to clustermessage.ClusterMessage.
func Response(body []byte, head *clustermessage.MessageHead) *clustermessage.ClusterMessage {
	msg := &clustermessage.ClusterMessage{
		Head:	head,
		Body:	body,
	}
	return msg
}

//ControlTaskResponse packages the body message to clustermessage.ControllerTaskResponse
//and serialize it.
func ControlTaskResponse(status int, body string) []byte {
	data := &clustermessage.ControllerTaskResponse{
		Timestamp:  time.Now().Unix(),
		StatusCode: int32(status),
		Body:       []byte(body),
	}

	resp, err := proto.Marshal(data)
	if err != nil {
		klog.Errorf("marshal ControllerTaskResponse failed: %v", err)
		return nil
	}
	return resp 
}

func GetControllerTaskFromClusterMessage(
	msg *clustermessage.ClusterMessage) *clustermessage.ControllerTask {
	if msg == nil {
		return nil
	}
	task := &clustermessage.ControllerTask{}
	err := proto.Unmarshal([]byte(msg.Body), task)
	if err != nil {
		klog.Errorf("unmarshal controller task failed: %v", err)
		return nil
	}
	return task
}

func GetControlMultiTaskFromClusterMessage(
	msg *clustermessage.ClusterMessage) *clustermessage.ControlMultiTask {
	if msg == nil {
		return nil
	}
	task := &clustermessage.ControlMultiTask{}
	err := proto.Unmarshal([]byte(msg.Body), task)
	if err != nil {
		klog.Errorf("unmarshal ControlMultiTask failed: %v", err)
		return nil
	}
	return task
}