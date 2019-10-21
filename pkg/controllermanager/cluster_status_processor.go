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

package controllermanager

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	otev1 "github.com/baidu/ote-stack/pkg/apis/ote/v1"
)

func (u *UpstreamProcessor) handleClusterStatusReport(clustername string, statusbody []byte) error {
	status, err := otev1.ClusterStatusDeserialize(statusbody)
	if err != nil {
		return fmt.Errorf("status body of cluster %s deserialize failed : %v", clustername, err)
	}

	return u.UpdateClusterStatus(clustername, status)
}

// UpdateClusterStatus update status of the given cluster.
func (u *UpstreamProcessor) UpdateClusterStatus(clustername string, status *otev1.ClusterStatus) error {
	cluster := &otev1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: otev1.ClusterNamespace,
			Name:      clustername,
		},
		Status: *status,
	}

	return u.clusterCRD.UpdateStatus(cluster)
}
