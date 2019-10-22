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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"

	otev1 "github.com/baidu/ote-stack/pkg/apis/ote/v1"
	oteclient "github.com/baidu/ote-stack/pkg/generated/clientset/versioned"
)

// ClusterCRD manipulates Cluster crd.
type ClusterCRD struct {
	client oteclient.Interface
}

// NewClusterCRD new a ClusterCRD with k8s client.
func NewClusterCRD(client oteclient.Interface) *ClusterCRD {
	return &ClusterCRD{client}
}

// Get get a Cluster by namespace and name.
func (c *ClusterCRD) Get(namespace, name string) *otev1.Cluster {
	cluster, err := c.client.OteV1().Clusters(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("get cluster(%s-%s) failed: %v", namespace, name, err)
		return nil
	}
	return cluster
}

// Create create a Cluster.
func (c *ClusterCRD) Create(cluster *otev1.Cluster) {
	_, err := c.client.OteV1().Clusters(cluster.ObjectMeta.Namespace).Create(cluster)
	if err != nil {
		klog.Errorf("create cluster(%v) failed: %v", cluster, err)
	}
}

// Delete delete a Cluster.
func (c *ClusterCRD) Delete(cluster *otev1.Cluster) {
	err := c.client.OteV1().Clusters(cluster.ObjectMeta.Namespace).Delete(
		cluster.ObjectMeta.Name, &metav1.DeleteOptions{})
	if err != nil {
		klog.Errorf("delete cluster(%v) failed: %v", cluster, err)
	}
}

// Update updates a Cluster.
func (c *ClusterCRD) Update(cluster *otev1.Cluster) {
	_, err := c.client.OteV1().Clusters(cluster.ObjectMeta.Namespace).Update(cluster)
	if err != nil {
		klog.Errorf("update cluster(%v) failed: %v", cluster, err)
	}
}

// UpdateStatus updates cluster status.
func (c *ClusterCRD) UpdateStatus(newcluster *otev1.Cluster) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		oldcluster, err := c.client.OteV1().Clusters(newcluster.Namespace).Get(newcluster.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("get original cluster(%s-%s) failed: %v",
				newcluster.Namespace, newcluster.Name, err)
		}

		if newcluster.Status.Timestamp <= oldcluster.Status.Timestamp {
			return fmt.Errorf("timestamp(%d) is too old while current timestamp is %d",
				newcluster.Status.Timestamp, oldcluster.Status.Timestamp)
		}

		oldcluster.Status = newcluster.Status
		_, err = c.client.OteV1().Clusters(oldcluster.Namespace).Update(oldcluster)
		return err
	})
}
