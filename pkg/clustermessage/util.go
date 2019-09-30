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

package clustermessage

import (
	"fmt"

	proto "github.com/golang/protobuf/proto"
)

// Serialize serializes a ClusterMessage to []byte, and return nil error if no error.
func (c *ClusterMessage) Serialize() ([]byte, error) {
	data, err := proto.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("serialize cluster message(%v) failed: %v", c, err)
	}
	return data, nil
}

// Deserialize deserializes data to a ClusterMessage, and return nil if no error.
func (c *ClusterMessage) Deserialize(data []byte) error {
	if data == nil {
		return fmt.Errorf("deserialize cluster message failed: data is nil")
	}
	err := proto.Unmarshal(data, c)
	if err != nil {
		return fmt.Errorf("deserialize cluster message(%s) failed: %v", string(data), err)
	}
	return nil
}

//ToClusterMessage makes ControllerTask to ClusterMessage.
func (c *ControllerTask) ToClusterMessage(head *MessageHead) (*ClusterMessage, error) {
	if head.Command != CommandType_ControlReq {
		return nil, fmt.Errorf("make ControllerTask to ClusterMessage failed: wrong command")
	}

	data, err := proto.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("make ControllerTask to ClusterMessage failed: %v", err)
	}

	ret := &ClusterMessage{
		Head: head,
		Body: data,
	}
	return ret, nil
}

//ToClusterMessage makes ControlMultiTask to ClusterMessage.
func (c *ControlMultiTask) ToClusterMessage(head *MessageHead) (*ClusterMessage, error) {
	if head.Command != CommandType_ControlMultiReq {
		return nil, fmt.Errorf("make ControlMultiTask to ClusterMessage failed: wrong command")
	}

	data, err := proto.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("make ControlMultiTask to ClusterMessage failed: %v", err)
	}

	ret := &ClusterMessage{
		Head: head,
		Body: data,
	}
	return ret, nil
}
