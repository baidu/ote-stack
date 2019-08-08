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

func clusterController2Pb(cc *otev1.ClusterController) *pb.ShimRequest {
	return &pb.ShimRequest{
		ParentClusterName: cc.Spec.ParentClusterName,
		Destination:       cc.Spec.Destination,
		Method:            cc.Spec.Method,
		URL:               cc.Spec.URL,
		Body:              cc.Spec.Body,
		Head: &pb.MessageHead{
			MessageID:         cc.ObjectMeta.Name,
			ParentClusterName: cc.Spec.ParentClusterName,
		},
	}
}

func pb2ClusterControllerStatus(in *pb.ShimResponse) (*pb.MessageHead, *otev1.ClusterControllerStatus) {
	return in.Head, &otev1.ClusterControllerStatus{
		Timestamp:  in.Timestamp,
		StatusCode: int(in.StatusCode),
		Body:       in.Body,
	}
}
