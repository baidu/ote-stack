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
	"testing"

	oteclient "github.com/baidu/ote-stack/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	otev1 "github.com/baidu/ote-stack/pkg/apis/ote/v1"
	"github.com/baidu/ote-stack/pkg/k8sclient"
)

func TestHandleClusterStatusReport(t *testing.T) {
	cluster1 := &otev1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: otev1.ClusterNamespace,
			Name:      "c1",
		},
		Status: otev1.ClusterStatus{
			Timestamp: 1571360000,
		},
	}

	processor := &UpstreamProcessor{
		clusterCRD: k8sclient.NewClusterCRD(oteclient.NewSimpleClientset(cluster1)),
	}

	testcase := []struct {
		Name        string
		ClusterName string
		ReportBody  []byte
		ExpectError bool
	}{
		{
			Name:        "success to update cluster status",
			ClusterName: "c1",
			ReportBody:  []byte(`{"timestamp":1571360001,"capacity":{"cpu":"16","memory":"12Gi"},"allocatable":{"cpu":"12","memory":"12Gi"}}`),
			ExpectError: false,
		},
		{
			Name:        "fail to update a non-existent cluster",
			ClusterName: "c2",
			ReportBody:  []byte(`{"capacity":{"cpu":"16"},"allocatable":{"cpu":"12"}}`),
			ExpectError: true,
		},
		{
			Name:        "fail to update a expired cluster status",
			ClusterName: "c1",
			ReportBody:  []byte(`{"timestamp":1571350000,"capacity":{"cpu":"16"},"allocatable":{"cpu":"12"}}`),
			ExpectError: true,
		},
	}

	for _, tc := range testcase {
		t.Run(tc.Name, func(t *testing.T) {
			assert := assert.New(t)
			err := processor.handleClusterStatusReport(tc.ClusterName, tc.ReportBody)
			if tc.ExpectError {
				assert.Error(err)
			} else {
				assert.NoError(err)
			}
		})
	}
}
