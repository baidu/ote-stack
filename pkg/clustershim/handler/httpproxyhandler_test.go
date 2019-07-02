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

	pb "github.com/baidu/ote-stack/pkg/clustershim/apis/v1"
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

func TestHTTPProxyHandlerDo(t *testing.T) {
	initTestServer()
	addr := testServer.Listener.Addr().String()

	successcase := []struct {
		Name       string
		Address    string
		Request    *pb.ShimRequest
		ExpectCode int32
	}{
		{
			Name:    "method Get",
			Address: addr,
			Request: &pb.ShimRequest{
				Method: http.MethodGet,
				URL:    "/",
			},
			ExpectCode: 200,
		},
		{
			Name:    "method Post",
			Address: addr,
			Request: &pb.ShimRequest{
				Method: http.MethodPost,
				URL:    "/",
			},
			ExpectCode: 200,
		},
		{
			Name:    "method PUT",
			Address: addr,
			Request: &pb.ShimRequest{
				Method: http.MethodPut,
				URL:    "/",
			},
			ExpectCode: 200,
		},
		{
			Name:    "method DELETE",
			Address: addr,
			Request: &pb.ShimRequest{
				Method: http.MethodDelete,
				URL:    "/",
			},
			ExpectCode: 200,
		},
	}

	for _, sc := range successcase {
		h := NewHTTPProxyHandler(sc.Address)
		resp, err := h.Do(sc.Request)
		if err != nil {
			t.Errorf("[%q] unexpected error %v", sc.Name, err)
		}

		if resp.StatusCode != sc.ExpectCode {
			t.Errorf("[%q] expected %v, got %v", sc.Name, sc.ExpectCode, resp.StatusCode)
		}
	}

	errorcase := []struct {
		Name    string
		Address string
		Request *pb.ShimRequest
	}{
		{
			Name:    "method not allowed",
			Address: addr,
			Request: &pb.ShimRequest{
				Method: http.MethodPatch,
				URL:    "/",
			},
		},
		{
			Name:    "address cannot reach",
			Address: "127.0.0.1:1234",
			Request: &pb.ShimRequest{
				Method: http.MethodGet,
				URL:    "/",
			},
		},
	}
	for _, ec := range errorcase {
		h := NewHTTPProxyHandler(ec.Address)
		_, err := h.Do(ec.Request)
		if err == nil {
			t.Errorf("[%q] expected error", ec.Name)
		}
	}
}
