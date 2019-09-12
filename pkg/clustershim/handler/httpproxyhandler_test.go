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

package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"

	"github.com/baidu/ote-stack/pkg/clustermessage"
)

var (
	testServer *httptest.Server
)

func initTestServer() {
	testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"code":200}`)
	}))
}

//getControllerTask get ControllerTask and serialize it
func getControllerTask(method string, t *testing.T) []byte {
	msg := &clustermessage.ControllerTask{
		Method: method,
		URI:    "/",
	}
	data, err := proto.Marshal(msg)
	if err != nil {
		t.Errorf("to controller task request failed: %v", err)
		return nil
	}
	return data	
}

func TestHTTPProxyHandlerDoControlRequest(t *testing.T) {
	initTestServer()
	addr := testServer.Listener.Addr().String()

	data1 := getControllerTask(http.MethodGet, t)
	data2 := getControllerTask(http.MethodPost, t)
	data3 := getControllerTask(http.MethodPut, t)
	data4 := getControllerTask(http.MethodDelete, t)
	data5 := getControllerTask(http.MethodPatch, t)

	successcase := []struct {
		Name       string
		Address    string
		Request    *clustermessage.ClusterMessage
		ExpectCode int32
	}{
		{
			Name:    "method Get",
			Address: addr,
			Request: &clustermessage.ClusterMessage{
				Head: &clustermessage.MessageHead{
					Command: clustermessage.CommandType_ControlReq,
				},
				Body: data1,
			},
			ExpectCode: 200,
		},
		{
			Name:    "method Post",
			Address: addr,
			Request: &clustermessage.ClusterMessage{
				Head: &clustermessage.MessageHead{
					Command: clustermessage.CommandType_ControlReq,
				},
				Body: data2,
			},
			ExpectCode: 200,
		},
		{
			Name:    "method PUT",
			Address: addr,
			Request: &clustermessage.ClusterMessage{
				Head: &clustermessage.MessageHead{
					Command: clustermessage.CommandType_ControlReq,
				},
				Body: data3,
			},
			ExpectCode: 200,
		},
		{
			Name:    "method DELETE",
			Address: addr,
			Request: &clustermessage.ClusterMessage{
				Head: &clustermessage.MessageHead{
					Command: clustermessage.CommandType_ControlReq,
				},
				Body: data4,
			},
			ExpectCode: 200,
		},
	}

	for _, sc := range successcase {
		h := NewHTTPProxyHandler(sc.Address).(*httpProxyHandler)
		resp, err := h.DoControlRequest(sc.Request)
		assert.Nil(t, err)

		task := &clustermessage.ControllerTaskResponse{}
		err = proto.Unmarshal([]byte(resp), task)
		if err != nil {
			t.Errorf("unmarshal controller task response failed: %v", err)
		}
		assert.Equal(t, sc.ExpectCode, task.StatusCode)
	}

	errorcase := []struct {
		Name    string
		Address string
		Request *clustermessage.ClusterMessage
	}{
		{
			Name:    "method not allowed",
			Address: addr,
			Request: &clustermessage.ClusterMessage{
				Head: &clustermessage.MessageHead{
					Command: clustermessage.CommandType_ControlReq,
				},
				Body: data5,
			},
		},
		{
			Name:    "address cannot reach",
			Address: "127.0.0.1:1234",
			Request: &clustermessage.ClusterMessage{
				Head: &clustermessage.MessageHead{
					Command: clustermessage.CommandType_ControlReq,
				},
				Body: data1,
			},
		},
	}
	for _, ec := range errorcase {
		h := NewHTTPProxyHandler(ec.Address).(*httpProxyHandler)
		_, err := h.DoControlRequest(ec.Request)
		assert.NotNil(t, err)
	}
}

func TestHTTPProxyHandlerDo(t *testing.T) {
	initTestServer()
	addr := testServer.Listener.Addr().String()

	//unsupportable command
	data := getControllerTask(http.MethodGet, t)
	msg := &clustermessage.ClusterMessage{
		Head: &clustermessage.MessageHead{
			Command: clustermessage.CommandType_NeighborRoute,
		},
		Body: data,
	}

	h := NewHTTPProxyHandler(addr)
	resp, err := h.Do(msg)
	assert.Nil(t, resp)
	assert.NotNil(t, err)
}