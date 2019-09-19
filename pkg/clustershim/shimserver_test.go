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
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"

	otev1 "github.com/baidu/ote-stack/pkg/apis/ote/v1"
	"github.com/baidu/ote-stack/pkg/clustermessage"
	"github.com/baidu/ote-stack/pkg/tunnel"
)

var (
	testShimServer *ShimServer
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
			return &clustermessage.ClusterMessage{Head: in.Head}, nil
		}

		msg := &clustermessage.ClusterMessage{
			Head: in.Head,
			Body: data,
		}
		return msg, nil
	case clustermessage.CommandType_ControlMultiReq:
		return nil, nil
	default:
		return nil, fmt.Errorf("command %s is not supported by ShimClient", in.Head.Command.String())
	}
}

func TestDo(t *testing.T) {
	server := NewShimServer()
	server.RegisterHandler(otev1.ClusterControllerDestAPI, &fakeShimHandler{})

	//unsupportable command
	data1 := getControllerTask(otev1.ClusterControllerDestAPI, "", "", t)

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
	server.RegisterHandler(otev1.ClusterControllerDestAPI, &fakeShimHandler{})

	data1 := getControllerTask(otev1.ClusterControllerDestAPI, "", "", t)

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

func newTestWSClient(s *http.Server, name string) *tunnel.WSClient {
	u := url.URL{
		Scheme: "ws",
		Host:   s.Addr,
		Path:   fmt.Sprintf("/%s/%s", shimServerPathForClusterController, name),
	}

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		fmt.Printf("newTestWSClient failed: %s", err.Error())
		return nil
	}
	return tunnel.NewWSClient("test", conn)
}
func TestClusterName(t *testing.T) {
	expectName := "test"
	ccclient := newTestWSClient(testShimServer.server, expectName)
	assert.NotNil(t, ccclient)

	if testShimServer.ccclient == nil {
		t.Errorf("testShimServer.ccclient unexpected nil")
		return
	}

	gotName := testShimServer.ClusterName()
	if gotName != expectName {
		t.Errorf("ClusterName expected %v, got %v", expectName, gotName)
	}

	ccclient.Close()
	time.Sleep(1 * time.Second)

}

func TestWriteMessage(t *testing.T) {
	ccclient := newTestWSClient(testShimServer.server, "test")
	sendChan := testShimServer.SendChan()

	if ccclient == nil {
		t.Errorf("cclient unexpected nil")
		return
	}

	if testShimServer.ccclient == nil {
		t.Errorf("testShimServer.ccclient unexpected nil")
		return
	}

	message := func(body string) *clustermessage.ClusterMessage {
		return &clustermessage.ClusterMessage{
			Head: &clustermessage.MessageHead{},
			Body: []byte(body),
		}
	}

	testcase := []struct {
		Name            string
		SendMessage     *clustermessage.ClusterMessage
		ExpectedMessage *clustermessage.ClusterMessage
		PreFunc         func()
	}{
		{
			Name:            "success to write message",
			SendMessage:     message("msg1"),
			ExpectedMessage: message("msg1"),
			PreFunc:         func() {},
		},
		{
			Name:            "cclient is closed",
			SendMessage:     message("msg2"),
			ExpectedMessage: &clustermessage.ClusterMessage{},
			PreFunc:         func() { ccclient.Close() },
		},
	}

	var data []byte
	go func() {
		var err error
		for {
			data, err = ccclient.ReadMessage()
			if err != nil {
				break
			}
		}
	}()

	for _, c := range testcase {
		c.PreFunc()
		sendChan <- *c.SendMessage
		time.Sleep(1 * time.Second)

		msg := &clustermessage.ClusterMessage{}
		if data != nil {
			err := msg.Deserialize(data)
			if err != nil {
				t.Errorf("[%q] unexpected error %v", c.Name, err)
				continue
			}
		}

		if !reflect.DeepEqual(c.ExpectedMessage, msg) {
			t.Errorf("[%q] expected %v, got %v", c.Name, c.ExpectedMessage, msg)
		}
	}
}

func TestMain(m *testing.M) {
	testShimServer = NewShimServer()
	go testShimServer.Serve("")
	time.Sleep(time.Second * 1)
	exit := m.Run()
	testShimServer.Close()
	os.Exit(exit)
}

func TestDoControlMultiRequest(t *testing.T) {
	server := NewShimServer()
	server.RegisterHandler(otev1.ClusterControllerDestAPI, &fakeShimHandler{})

	//supportable handler
	data1 := makeControlMultiTask(otev1.ClusterControllerDestAPI, t)
	msg1 := clustermessage.ClusterMessage{
		Head: &clustermessage.MessageHead{
			Command: clustermessage.CommandType_ControlMultiReq,
		},
		Body: data1,
	}
	err := server.DoControlMultiRequest(&msg1)
	assert.Nil(t, err)

	//unsupportable handler
	data2 := makeControlMultiTask(DestNoHandler, t)
	msg2 := clustermessage.ClusterMessage{
		Head: &clustermessage.MessageHead{
			Command: clustermessage.CommandType_ControlMultiReq,
		},
		Body: data2,
	}
	err = server.DoControlMultiRequest(&msg2)
	assert.NotNil(t, err)

	//unsupportable command
	msg3 := clustermessage.ClusterMessage{
		Head: &clustermessage.MessageHead{
			Command: clustermessage.CommandType_NeighborRoute,
		},
		Body: data1,
	}
	err = server.DoControlMultiRequest(&msg3)
	assert.NotNil(t, err)
}
