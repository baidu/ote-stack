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
	"bufio"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	fakerest "k8s.io/client-go/rest/fake"
	"github.com/golang/protobuf/proto"

	"github.com/baidu/ote-stack/pkg/clustermessage"
	//	fakek8s "github.com/baidu/ote-stack/pkg/generated/clientset/versioned/fake"
)

func makeControlMultiTask(method string, t *testing.T) []byte {
	msg := &clustermessage.ControlMultiTask{
		Method: method,
		URI:    "/",
		Body:	[][]byte{{1},{2}},
	}
	data, err := proto.Marshal(msg)
	if err != nil {
		t.Errorf("to controller task request failed: %v", err)
		return nil
	}
	return data	
}

func TestK8sHandlerDo(t *testing.T) {
	fakeRestClient := &fakerest.RESTClient{
		Client: fakerest.CreateHTTPClient(
			func(req *http.Request) (*http.Response, error) {
				body := "HTTP/1.0 200 OK\r\nConnection: close\r\n\r\nOK\n"
				resp, _ := http.ReadResponse(bufio.NewReader(strings.NewReader(body)), req)
				return resp, nil
			},
		),
		GroupVersion:         v1.SchemeGroupVersion,
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		VersionedAPIPath:     "/",
	}
	h := &k8sHandler{restclient: fakeRestClient}

	//unsupportable command
	data := getControllerTask(http.MethodGet, t)
	msg := &clustermessage.ClusterMessage{
		Head: &clustermessage.MessageHead{
			Command: clustermessage.CommandType_NeighborRoute,
		},
		Body: data,
	}

	resp, err := h.Do(msg)
	assert.Nil(t, resp)
	assert.NotNil(t, err)
}

func TestK8sHandlerDoControlRequest(t *testing.T) {
	fakeRestClient := &fakerest.RESTClient{
		Client: fakerest.CreateHTTPClient(
			func(req *http.Request) (*http.Response, error) {
				body := "HTTP/1.0 200 OK\r\nConnection: close\r\n\r\nOK\n"
				resp, _ := http.ReadResponse(bufio.NewReader(strings.NewReader(body)), req)
				return resp, nil
			},
		),
		GroupVersion:         v1.SchemeGroupVersion,
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		VersionedAPIPath:     "/",
	}
	h := &k8sHandler{restclient: fakeRestClient}
	
	data1 := getControllerTask(http.MethodGet, t)
	data2 := getControllerTask(http.MethodPost, t)
	data3 := getControllerTask(http.MethodPut, t)
	data4 := getControllerTask(http.MethodDelete, t)
	data5 := getControllerTask(http.MethodPatch, t)

	successcase := []struct {
		Name       string
		Request    *clustermessage.ClusterMessage
		ExpectCode int32
	}{
		{
			Name: "method Get",
			Request: &clustermessage.ClusterMessage{
				Head: &clustermessage.MessageHead{
					Command: clustermessage.CommandType_ControlReq,
				},
				Body: data1,
			},
			ExpectCode: 200,
		},
		{
			Name: "method Post",
			Request: &clustermessage.ClusterMessage{
				Head: &clustermessage.MessageHead{
					Command: clustermessage.CommandType_ControlReq,
				},
				Body: data2,
			},
			ExpectCode: 200,
		},
		{
			Name: "method PUT",
			Request: &clustermessage.ClusterMessage{
				Head: &clustermessage.MessageHead{
					Command: clustermessage.CommandType_ControlReq,
				},
				Body: data3,
			},
			ExpectCode: 200,
		},
		{
			Name: "method DELETE",
			Request: &clustermessage.ClusterMessage{
				Head: &clustermessage.MessageHead{
					Command: clustermessage.CommandType_ControlReq,
				},
				Body: data4,
			},
			ExpectCode: 200,
		},
		{
			Name: "method PATCH",
			Request: &clustermessage.ClusterMessage{
				Head: &clustermessage.MessageHead{
					Command: clustermessage.CommandType_ControlReq,
				},
				Body: data5,
			},
			ExpectCode: 200,
		},
	}

	for _, sc := range successcase {
		resp, err := h.DoControlRequest(sc.Request)
		assert.Nil(t, err)

		task := &clustermessage.ControllerTaskResponse{}
		err = proto.Unmarshal([]byte(resp), task)
		if err != nil {
			t.Errorf("unmarshal controller task response failed: %v", err)
		}
		assert.Equal(t, sc.ExpectCode, task.StatusCode)
	}

	data6 := getControllerTask("", t)
	errorcase := []struct {
		Name    string
		Request *clustermessage.ClusterMessage
	}{
		{
			Name: "no method",
			Request: &clustermessage.ClusterMessage{
				Head: &clustermessage.MessageHead{
					Command: clustermessage.CommandType_NeighborRoute,
				},
				Body: data6,
			},
		},
	}
	for _, ec := range errorcase {
		_, err := h.DoControlRequest(ec.Request)
		assert.NotNil(t, err)
	}
}

func TestDoControlMultiRequest(t *testing.T) {
	fakeRestClient := &fakerest.RESTClient{
		Client: fakerest.CreateHTTPClient(
			func(req *http.Request) (*http.Response, error) {
				body := "HTTP/1.0 200 OK\r\nConnection: close\r\n\r\nOK\n"
				resp, _ := http.ReadResponse(bufio.NewReader(strings.NewReader(body)), req)
				return resp, nil
			},
		),
		GroupVersion:         v1.SchemeGroupVersion,
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		VersionedAPIPath:     "/",
	}
	h := &k8sHandler{restclient: fakeRestClient}

	data1 := makeControlMultiTask(http.MethodGet, t)
	data2 := makeControlMultiTask(http.MethodPost, t)
	data3 := makeControlMultiTask(http.MethodPut, t)
	data4 := makeControlMultiTask(http.MethodDelete, t)
	data5 := makeControlMultiTask(http.MethodPatch, t)

	successcase := []struct {
		Name       string
		Request    *clustermessage.ClusterMessage
	}{
		{
			Name: "method Get",
			Request: &clustermessage.ClusterMessage{
				Head: &clustermessage.MessageHead{
					Command: clustermessage.CommandType_ControlMultiReq,
				},
				Body: data1,
			},
		},
		{
			Name: "method Post",
			Request: &clustermessage.ClusterMessage{
				Head: &clustermessage.MessageHead{
					Command: clustermessage.CommandType_ControlMultiReq,
				},
				Body: data2,
			},
		},
		{
			Name: "method PUT",
			Request: &clustermessage.ClusterMessage{
				Head: &clustermessage.MessageHead{
					Command: clustermessage.CommandType_ControlMultiReq,
				},
				Body: data3,
			},
		},
		{
			Name: "method DELETE",
			Request: &clustermessage.ClusterMessage{
				Head: &clustermessage.MessageHead{
					Command: clustermessage.CommandType_ControlMultiReq,
				},
				Body: data4,
			},
		},
		{
			Name: "method PATCH",
			Request: &clustermessage.ClusterMessage{
				Head: &clustermessage.MessageHead{
					Command: clustermessage.CommandType_ControlMultiReq,
				},
				Body: data5,
			},
		},
	}

	for _, sc := range successcase {
		err := h.DoControlMultiRequest(sc.Request)
		assert.Nil(t, err)
	}

	data6 := makeControlMultiTask("", t)
	errorcase := []struct {
		Name    string
		Request *clustermessage.ClusterMessage
	}{
		{
			Name: "no method",
			Request: &clustermessage.ClusterMessage{
				Head: &clustermessage.MessageHead{
					Command: clustermessage.CommandType_ControlMultiReq,
				},
				Body: data6,
			},
		},
	}
	for _, ec := range errorcase {
		err := h.DoControlMultiRequest(ec.Request)
		assert.NotNil(t, err)
	}
}