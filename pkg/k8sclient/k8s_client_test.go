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

package k8sclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	otev1 "github.com/baidu/ote-stack/pkg/apis/ote/v1"
	oteclient "github.com/baidu/ote-stack/pkg/generated/clientset/versioned/fake"
)

func TestClusterCRD(t *testing.T) {
	cluster1 := &otev1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "c1",
		},
	}
	cluster2 := &otev1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "c2",
		},
	}
	client := oteclient.NewSimpleClientset(cluster1)

	clusterCRD := NewClusterCRD(client)
	assert.NotNil(t, clusterCRD)
	o := clusterCRD.Get("default", "c1")
	assert.NotNil(t, o)

	o = clusterCRD.Get("default", "c2")
	assert.Nil(t, o)
	clusterCRD.Create(cluster2)
	o = clusterCRD.Get("default", "c2")
	assert.NotNil(t, o)

	clusterCRD.Delete(cluster1)
	o = clusterCRD.Get("default", "c1")
	assert.Nil(t, o)
}

func TestClusterControllerCRD(t *testing.T) {
	clustercontroller1 := &otev1.ClusterController{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "cc1",
		},
	}
	client := oteclient.NewSimpleClientset(clustercontroller1)

	clustercontrollerCRD := NewClusterControllerCRD(client)
	assert.NotNil(t, clustercontrollerCRD)
	o := clustercontrollerCRD.Get("default", "cc1")
	assert.NotNil(t, o)

	o.Spec.Destination = "isset"
	clustercontrollerCRD.Update(o)
	o = clustercontrollerCRD.Get("default", "cc1")
	assert.Equal(t, "isset", o.Spec.Destination)
}
