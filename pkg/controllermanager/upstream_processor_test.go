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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/baidu/ote-stack/pkg/clustermessage"
)

func TestHandleReceivedMessage(t *testing.T) {
	u := NewUpstreamProcessor(&K8sContext{})
	// get msg failed
	err := u.HandleReceivedMessage("", nil)
	assert.NotNil(t, err)

	// get msg with nil Head
	msg := &clustermessage.ClusterMessage{}
	data, err := msg.Serialize()
	assert.NotNil(t, data)
	assert.Nil(t, err)
	err = u.HandleReceivedMessage("", data)
	assert.NotNil(t, err)

	// get msg with command not supported(Reserved)
	msg.Head = &clustermessage.MessageHead{Command: clustermessage.CommandType_Reserved}
	data, err = msg.Serialize()
	assert.NotNil(t, data)
	assert.Nil(t, err)
	err = u.HandleReceivedMessage("", data)
	assert.NotNil(t, err)

	// get msg with command EdgeReport
	// TODO detail assert
	msg.Head.Command = clustermessage.CommandType_EdgeReport
	data, err = msg.Serialize()
	assert.NotNil(t, data)
	assert.Nil(t, err)
	err = u.HandleReceivedMessage("", data)
	assert.Nil(t, err)
}
