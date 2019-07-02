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

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	fakerest "k8s.io/client-go/rest/fake"

	pb "github.com/baidu/ote-stack/pkg/clustershim/apis/v1"
	//	fakek8s "github.com/baidu/ote-stack/pkg/generated/clientset/versioned/fake"
)

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

	successcase := []struct {
		Name       string
		Request    *pb.ShimRequest
		ExpectCode int32
	}{
		{
			Name: "method Get",
			Request: &pb.ShimRequest{
				Method: http.MethodGet,
				URL:    "/",
			},
			ExpectCode: 200,
		},
		{
			Name: "method Post",
			Request: &pb.ShimRequest{
				Method: http.MethodPost,
				URL:    "/",
			},
			ExpectCode: 200,
		},
		{
			Name: "method PUT",
			Request: &pb.ShimRequest{
				Method: http.MethodPut,
				URL:    "/",
			},
			ExpectCode: 200,
		},
		{
			Name: "method DELETE",
			Request: &pb.ShimRequest{
				Method: http.MethodDelete,
				URL:    "/",
			},
			ExpectCode: 200,
		},
		{
			Name: "method PATCH",
			Request: &pb.ShimRequest{
				Method: http.MethodPatch,
				URL:    "/",
			},
			ExpectCode: 200,
		},
	}

	for _, sc := range successcase {
		resp, err := h.Do(sc.Request)
		if err != nil {
			t.Errorf("[%q] unexpected error %v", sc.Name, err)
		}

		if resp.StatusCode != sc.ExpectCode {
			t.Errorf("[%q] expected %v, got %v", sc.Name, sc.ExpectCode, resp.StatusCode)
		}
	}
}
