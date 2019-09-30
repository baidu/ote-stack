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

//Package controller provides some method to help controller handles event.
package controller

import (
	"encoding/json"
	"fmt"
)

const (
	OteNamespaceKind = "Namespace"
	OteNamespaceURI  = "/api/v1/namespaces"
	OteApiVersionV1  = "v1"
)

//oteNamespace is responsible for constructing namespace object.
type oteNamespace struct {
	Kind       string            `json:"kind"`
	ApiVersion string            `json:"apiVersion"`
	MetaData   map[string]string `json:"metadata"`
}

//SerializeNamespaceObject serializes an oteNamespace to be the body of
//k8s rest request with specific name.
func SerializeNamespaceObject(name string) ([]byte, error) {
	data := make(map[string]string)
	data["name"] = name
	msg := oteNamespace{
		Kind:       OteNamespaceKind,
		ApiVersion: OteApiVersionV1,
		MetaData:   data,
	}
	ret, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal Namespace Object failed.")
	}
	return ret, nil
}
