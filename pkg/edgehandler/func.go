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

package edgehandler

import (
	otev1 "github.com/baidu/ote-stack/pkg/apis/ote/v1"
	pb "github.com/baidu/ote-stack/pkg/clustershim/apis/v1"
)

func pb2ClusterControllerSpec(in *pb.ShimRequest) *otev1.ClusterControllerSpec {
	return &otev1.ClusterControllerSpec{
		ParentClusterName: in.ParentClusterName,
		Destination:       in.Destination,
		Method:            in.Method,
		URL:               in.URL,
		Body:              in.Body,
	}
}

func clusterControllerSpec2Pb(spec *otev1.ClusterControllerSpec) *pb.ShimRequest {
	return &pb.ShimRequest{
		ParentClusterName: spec.ParentClusterName,
		Destination:       spec.Destination,
		Method:            spec.Method,
		URL:               spec.URL,
		Body:              spec.Body,
	}
}

func clusterControllerStatus2Pb(status *otev1.ClusterControllerStatus) *pb.ShimResponse {
	return &pb.ShimResponse{
		Timestamp:  status.Timestamp,
		StatusCode: int32(status.StatusCode),
		Body:       status.Body,
	}

}

func pb2ClusterControllerStatus(in *pb.ShimResponse) *otev1.ClusterControllerStatus {
	return &otev1.ClusterControllerStatus{
		Timestamp:  in.Timestamp,
		StatusCode: int(in.StatusCode),
		Body:       in.Body,
	}
}
