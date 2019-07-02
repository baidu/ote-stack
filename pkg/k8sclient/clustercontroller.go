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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	otev1 "github.com/baidu/ote-stack/pkg/apis/ote/v1"
	oteclient "github.com/baidu/ote-stack/pkg/generated/clientset/versioned"
)

// ClusterControllerCRD manipulates ClusterController crd.
type ClusterControllerCRD struct {
	client oteclient.Interface
}

// NewClusterControllerCRD new a ClusterControllerCRD with k8s client.
func NewClusterControllerCRD(client oteclient.Interface) *ClusterControllerCRD {
	return &ClusterControllerCRD{client}
}

// Get get a ClusterController by namespace and name.
func (c *ClusterControllerCRD) Get(namespace, name string) *otev1.ClusterController {
	cc, err := c.client.OteV1().ClusterControllers(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("get clustercontroller(%s-%s) failed: %v", namespace, name, err)
		return nil
	}
	return cc
}

// Update update a ClusterControllers.
func (c *ClusterControllerCRD) Update(cc *otev1.ClusterController) {
	_, err := c.client.OteV1().ClusterControllers(cc.ObjectMeta.Namespace).Update(cc)
	if err != nil {
		klog.Errorf("update clustercontroller(%v) failed: %v", cc, err)
	}
}
