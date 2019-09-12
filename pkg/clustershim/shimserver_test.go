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
	"testing"
	"time"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	
	"github.com/baidu/ote-stack/pkg/clustermessage"
)

type fakeShimHandler struct{}

func (f *fakeShimHandler) Do(in *clustermessage.ClusterMessage) (*clustermessage.ClusterMessage, error) {
	switch in.Head.Command {
	case clustermessage.CommandType_ControlReq:
		resp := &clustermessage.ControllerTaskResponse{
			Timestamp:  time.Now().Unix(),
			StatusCode: 200,
			Body:       []byte(""),
		}
	
		data, err := proto.Marshal(resp)
		if err != nil {
			fmt.Errorf("shim resp to controller task resp failed: %v", err)
			return &clustermessage.ClusterMessage{Head: in.Head,}, nil
		}
	
		msg := &clustermessage.ClusterMessage{
			Head: in.Head,
			Body: data,
		}
		return msg, nil
	default:
		return nil, fmt.Errorf("command %s is not supported by ShimClient", in.Head.Command.String())
	} 
}

func TestDo(t *testing.T) {
	server := NewShimServer()
	server.RegisterHandler("api", &fakeShimHandler{})

	//unsupportable command
	data1 := getControllerTask("api", "", "", t)
	
	msg := &clustermessage.ClusterMessage{
		Head: &clustermessage.MessageHead{
			Command: clustermessage.CommandType_NeighborRoute,
		}, 
		Body: data1,
	}

	resp, err := server.Do(msg)
	assert.Nil(t, resp)
	assert.NotNil(t, err)
}

func TestDoControlRequest(t *testing.T) {
	server := NewShimServer()
	server.RegisterHandler("api", &fakeShimHandler{})

	data1 := getControllerTask("api", "", "", t)
	
	successcase := []struct {
		Name       string
		Request    *clustermessage.ClusterMessage
		ExpectCode int32
	}{
		{
			Name: "success",
			Request: &clustermessage.ClusterMessage{
				Head: &clustermessage.MessageHead{
					Command: clustermessage.CommandType_ControlReq,
				}, 
				Body: data1,
			},
			ExpectCode: 200,
		},
	}

	for _, sc := range successcase {
		resp, err := server.DoControlRequest(sc.Request)
		assert.Nil(t, err)

		task := &clustermessage.ControllerTaskResponse{}
		err = proto.Unmarshal([]byte(resp.Body), task)
		if err != nil {
			t.Errorf("unmarshal controller task response failed: %v", err)
		}
		assert.Equal(t, sc.ExpectCode, task.StatusCode)
	}

	data2 := getControllerTask("test", "", "", t)

	errorcase := []struct {
		Name    string
		Request *clustermessage.ClusterMessage
	}{
		{
			Name: "no handler",
			Request: &clustermessage.ClusterMessage{
				Head: &clustermessage.MessageHead{
					Command: clustermessage.CommandType_ControlReq,
				},
				Body: data2,
			},
		},
		{
			Name: "no command",
			Request: &clustermessage.ClusterMessage{
				Head: &clustermessage.MessageHead{
					Command: clustermessage.CommandType_NeighborRoute,
				},
				Body: data1,
			},
		},
	}

	for _, ec := range errorcase {
		resp, err := server.DoControlRequest(ec.Request)
		assert.NotNil(t, err)
		if ec.Name == "no command" {
			assert.Nil(t, resp)
		} else {
			assert.NotNil(t, resp)
		}
	}
}