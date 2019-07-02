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

// Package handler provides the ability to interact with k8s or third-party services.
package handler

import (
	"time"

	pb "github.com/baidu/ote-stack/pkg/clustershim/apis/v1"
)

// Handler is shim handler interface that contains
// the methods required to interact to remote server.
type Handler interface {
	// Do handle the request and transmit to corresponding server.
	Do(*pb.ShimRequest) (*pb.ShimResponse, error)
}

// Response packages the body message to pb.ShimResponse.
func Response(status int, body string) *pb.ShimResponse {
	return &pb.ShimResponse{
		Timestamp:  time.Now().Unix(),
		StatusCode: int32(status),
		Body:       body,
	}
}
