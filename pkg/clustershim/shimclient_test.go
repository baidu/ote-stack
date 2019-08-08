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

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	otev1 "github.com/baidu/ote-stack/pkg/apis/ote/v1"
	pb "github.com/baidu/ote-stack/pkg/clustershim/apis/v1"
	"github.com/baidu/ote-stack/pkg/config"
	oteclient "github.com/baidu/ote-stack/pkg/generated/clientset/versioned/fake"
)

func TestShimClientDo(t *testing.T) {
	c := &config.ClusterControllerConfig{
		K8sClient: oteclient.NewSimpleClientset(
			&otev1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "c1",
				},
			},
		),
		HelmTillerAddr: "",
	}
	localClient := NewlocalShimClient(c)
	assert.Nil(t, localClient.ReturnChan())

	// no handler
	in := pb.ShimRequest{
		Destination: "nohandler",
		Method:      "GET",
		URL:         "/apis/ote.baidu.com/v1/namespaces/default/clusters",
	}
	resp, err := localClient.Do(&in)
	assert.NotNil(t, resp) // local shim client return not nil resp
	assert.NotNil(t, err)
}
