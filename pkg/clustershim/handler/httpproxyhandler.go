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
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
		  
	"github.com/baidu/ote-stack/pkg/clustermessage"
)

const (
	connectTimeout = 60
	prefixHTTP     = "http://"
	prefixHTTPS    = "https://"
)

type httpProxyHandler struct {
	client *http.Client
	addr   string
}

// NewHTTPProxyHandler returns a new httpProxyHandler.
func NewHTTPProxyHandler(address string) Handler {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	cl := &http.Client{
		Timeout:   time.Second * connectTimeout,
		Transport: tr,
	}

	if !strings.HasPrefix(address, prefixHTTP) && !strings.HasPrefix(address, prefixHTTPS) {
		address = prefixHTTP + address
	}

	return &httpProxyHandler{
		client: cl,
		addr:   address,
	}
}

func (h *httpProxyHandler) Do(in *clustermessage.ClusterMessage) (*clustermessage.ClusterMessage, error) {
	switch in.Head.Command {
	case clustermessage.CommandType_ControlReq:
		resp, err := h.DoControlRequest(in)
		return Response(resp, in.Head), err
	default:
		return nil, fmt.Errorf("command %s is not supported by httpProxyHandler", in.Head.Command.String())
	}
}

func (h *httpProxyHandler) DoControlRequest(in *clustermessage.ClusterMessage) ([]byte, error) {
	var req *http.Request
	var err error

	controllerTask := GetControllerTaskFromClusterMessage(in)
	if controllerTask == nil {
		return ControlTaskResponse(http.StatusNotFound, ""), fmt.Errorf("Controllertask Not Found")
	}

	url := h.addr + controllerTask.URI
	buf := bytes.NewBuffer([]byte(controllerTask.Body))

	switch controllerTask.Method {
	case http.MethodGet:
		req, err = http.NewRequest(http.MethodGet, url, buf)
	case http.MethodPost:
		req, err = http.NewRequest(http.MethodPost, url, buf)
	case http.MethodDelete:
		req, err = http.NewRequest(http.MethodDelete, url, buf)
	case http.MethodPut:
		req, err = http.NewRequest(http.MethodPut, url, buf)
	default:
		return ControlTaskResponse(http.StatusMethodNotAllowed, ""), fmt.Errorf("method not allowed")
	}

	if err != nil {
		return ControlTaskResponse(http.StatusInternalServerError, ""), err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := h.client.Do(req)
	if err != nil {
		return ControlTaskResponse(http.StatusInternalServerError, ""), err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ControlTaskResponse(http.StatusInternalServerError, ""), err
	}

	return ControlTaskResponse(resp.StatusCode, string(body)), nil
}