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

	pb "github.com/baidu/ote-stack/pkg/clustershim/apis/v1"
)

type fakeShimHandler struct{}

func (f *fakeShimHandler) Do(in *pb.ShimRequest) (*pb.ShimResponse, error) {
	resp := &pb.ShimResponse{
		Timestamp:  time.Now().Unix(),
		StatusCode: 200,
		Body:       "",
	}
	return resp, nil
}

func TestDo(t *testing.T) {
	server := NewShimServer()
	server.RegisterHandler("api", &fakeShimHandler{})
	successcase := []struct {
		Name       string
		Request    *pb.ShimRequest
		ExpectCode int32
	}{
		{
			Name: "success",
			Request: &pb.ShimRequest{
				Destination: "api",
			},
			ExpectCode: 200,
		},
	}

	for _, sc := range successcase {
		resp, err := server.Do(sc.Request)
		if err != nil {
			t.Errorf("[%q] unexpected error %v", sc.Name, err)
		}

		if resp.StatusCode != sc.ExpectCode {
			t.Errorf("[%q] expected %v, got %v", sc.Name, sc.ExpectCode, resp.StatusCode)
		}
	}

	errorcase := []struct {
		Name    string
		Request *pb.ShimRequest
	}{
		{
			Name: "no handler",
			Request: &pb.ShimRequest{
				Destination: "test",
			},
		},
	}

	for _, ec := range errorcase {
		_, err := server.Do(ec.Request)
		if err == nil {
			t.Errorf("[%q] expected error", ec.Name)
		}
	}
}
