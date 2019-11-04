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

package v1

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterStatusRegist describe a cluster registered status,
// should be set to Cluster.Spec.Status if a cluster've registered.
const (
	ClusterStatusRegist = "regist"
)

// ClusterControllerDest* describe the way to process ClusterController,
// should be set to ClusterController.Spec.Destination.
const (
	ClusterControllerDestAPI             = "api"      // sent to k8s apiserver
	ClusterControllerDestHelm            = "helm"     // sent to helm
	ClusterControllerDestRegistCluster   = "regist"   // cluster regist
	ClusterControllerDestUnregistCluster = "unregist" // cluster unregist
	ClusterControllerDestClusterRoute    = "route"    // cluster route
	ClusterControllerDestClusterSubtree  = "subtree"  // cluster subtree

	ClusterStatusOnline  = "online"
	ClusterStatusOffline = "offline"
)

// ClusterNamespace defines the namespace of k8s crd must be in.
// CRD out of the namespace won't be watched.
const (
	ClusterNamespace = "kube-system"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterController is the k8s crd to manipulate all clusters in ote.
type ClusterController struct {
	metav1.TypeMeta   `json:", inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterControllerSpec              `json:"spec"`
	Status map[string]ClusterControllerStatus `json:"status"`
}

// ClusterControllerSpec is specification of a ClusterController.
type ClusterControllerSpec struct {
	ParentClusterName string `json:"parentClusterName"`

	ClusterSelector string `json:"clusterSelector"`
	Destination     string `json:"destination"`
	Method          string `json:"method"`
	URL             string `json:"url"`

	Body string `json:"body"`
}

// ClusterControllerStatus is status of a ClusterController.
type ClusterControllerStatus struct {
	Timestamp  int64  `json:"timestamp"`
	StatusCode int    `json:"code"`
	Body       string `json:"body"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterControllerList is a list of ClusterController.
type ClusterControllerList struct {
	metav1.TypeMeta `json:", inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ClusterController `json:"items,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Cluster is the k8s crd to store cluster info.
type Cluster struct {
	metav1.TypeMeta   `json:", inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec"`
	Status ClusterStatus `json:"status"`
}

// ClusterSpec is specification of a Cluster.
type ClusterSpec struct {
	Name string `json:"name"`
}

// ClusterStatus is status of a Cluster.
type ClusterStatus struct {
	Listen     string `json:"listen,omitempty"`
	ParentName string `json:"parentName,omitempty"`
	Status     string `json:"status,omitempty"`
	Timestamp  int64  `json:"timestamp"`
	ClusterResource
}

// ClusterResource represents the resources of a cluster.
type ClusterResource struct {
	// Capacity represents the total resources of a cluster.
	Capacity map[corev1.ResourceName]*resource.Quantity `json:"capacity,omitempty"`
	// Allocatable represents the resources of a cluster that are available for scheduling.
	Allocatable map[corev1.ResourceName]*resource.Quantity `json:"allocatable,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterList is a list of Cluster.
type ClusterList struct {
	metav1.TypeMeta `json:", inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items,omitempty"`
}

// Serialize serialize ClusterController using json.
func (cc *ClusterController) Serialize() ([]byte, error) {
	b, err := json.Marshal(cc)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// ClusterControllerDeserialize deserialize ClusterController using json.
func ClusterControllerDeserialize(b []byte) (*ClusterController, error) {
	cc := ClusterController{}
	err := json.Unmarshal(b, &cc)
	if err != nil {
		return nil, err
	}
	return &cc, nil
}

// Serialize serialize Cluster using json.
func (c *Cluster) Serialize() ([]byte, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// ClusterDeserialize deserialize Cluster using json.
func ClusterDeserialize(b []byte) (*Cluster, error) {
	c := Cluster{}
	err := json.Unmarshal(b, &c)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Serialize serialize ClusterStatus using json.
func (c *ClusterStatus) Serialize() ([]byte, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// ClusterStatusDeserialize deserialize ClusterStatus using json.
func ClusterStatusDeserialize(b []byte) (*ClusterStatus, error) {
	c := ClusterStatus{}
	err := json.Unmarshal(b, &c)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// WrapperToClusterController wrapper a Cluster to a ClusterController using json.
func (c *Cluster) WrapperToClusterController(dst string) (*ClusterController, error) {
	cbyte, err := c.Serialize()
	if err != nil {
		return nil, fmt.Errorf("serialize cluster crd(%v) failed: %v", c, err)
	}
	cc := ClusterController{
		Spec: ClusterControllerSpec{
			Destination: dst,
			Body:        string(cbyte),
		},
	}
	return &cc, nil
}
