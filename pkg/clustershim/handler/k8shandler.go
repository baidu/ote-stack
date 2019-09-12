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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/baidu/ote-stack/pkg/clustermessage"
)

type k8sHandler struct {
	restclient rest.Interface
}

// NewK8sHandler returns a new k8sHandler.
func NewK8sHandler(cl kubernetes.Interface) Handler {
	return &k8sHandler{restclient: cl.Discovery().RESTClient()}
}

func (k *k8sHandler) Do(in *clustermessage.ClusterMessage) (*clustermessage.ClusterMessage, error) {
	switch in.Head.Command {
	case clustermessage.CommandType_ControlReq:
		resp, err := k.DoControlRequest(in)
		return Response(resp, in.Head), err
	default:
		return nil, fmt.Errorf("command %s is not supported by k8sHandler", in.Head.Command.String())
	}
}

func (k *k8sHandler) DoControlRequest(in *clustermessage.ClusterMessage) ([]byte, error) {
	var req *rest.Request

	controllerTask := GetControllerTaskFromClusterMessage(in)
	if controllerTask == nil {
		return ControlTaskResponse(http.StatusNotFound, ""), fmt.Errorf("Controllertask Not Found")
	}

	switch controllerTask.Method {
	case http.MethodGet, http.MethodPost, http.MethodDelete, http.MethodPut:
		req = k.restclient.Verb(controllerTask.Method)
	case http.MethodPatch:
		req = k.restclient.Patch(types.JSONPatchType)
	default:
		return ControlTaskResponse(http.StatusMethodNotAllowed, ""), fmt.Errorf("method not allowed")
	}

	req.Body([]byte(controllerTask.Body))
	req.RequestURI(controllerTask.URI)

	result := req.Do()

	var code int
	result.StatusCode(&code)

	raw, _ := result.Raw()

	return ControlTaskResponse(code, string(raw)), nil
}
