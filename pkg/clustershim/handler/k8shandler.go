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

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"

	pb "github.com/baidu/ote-stack/pkg/clustershim/apis/v1"
	oteclient "github.com/baidu/ote-stack/pkg/generated/clientset/versioned"
)

type k8sHandler struct {
	restclient rest.Interface
}

// NewK8sHandler returns a new k8sHandler.
func NewK8sHandler(cl oteclient.Interface) Handler {
	return &k8sHandler{restclient: cl.Discovery().RESTClient()}
}

func (k *k8sHandler) Do(in *pb.ShimRequest) (*pb.ShimResponse, error) {

	var req *rest.Request
	switch in.Method {
	case http.MethodGet, http.MethodPost, http.MethodDelete, http.MethodPut:
		req = k.restclient.Verb(in.Method)
	case http.MethodPatch:
		req = k.restclient.Patch(types.JSONPatchType)
	default:
		return Response(http.StatusMethodNotAllowed, ""), fmt.Errorf("method not allowed")
	}

	req.Body([]byte(in.Body))
	req.RequestURI(in.URL)
	result := req.Do()

	var code int
	result.StatusCode(&code)

	raw, _ := result.Raw()

	return Response(code, string(raw)), nil
}
