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

// Package config defines data structure needed by cluster controller,
// and const of cluster controller.
package config

import (
	"encoding/json"
	"fmt"

	otev1 "github.com/baidu/ote-stack/pkg/apis/ote/v1"
	oteclient "github.com/baidu/ote-stack/pkg/generated/clientset/versioned"
)

const (
	// RootClusterName defines the cluster name of root cluster.
	RootClusterName = "Root"

	// ClusterConnectHeaderListenAddr defines request header to post listen address of the child.
	// For edge when connect to parent, set this header to address listened by the cluster,
	// so let parent know the address of child, thus a cluster can get its neighbor from parent.
	ClusterConnectHeaderListenAddr = "listen-addr"
	// ClusterConnectHeaderUserDefineName is the user-define name of the child
	ClusterConnectHeaderUserDefineName = "name"

	// K8sInformerSyncDuration defines k8s informer sync seconds.
	K8sInformerSyncDuration = 10
)

// ErrDuplicatedName is error message format.
var (
	ErrDuplicatedName = fmt.Errorf("cluster name duplicated")
)

// ClusterControllerConfig contains config needed by cluster controller.
type ClusterControllerConfig struct {
	TunnelListenAddr      string
	ParentCluster         string
	ClusterName           string
	ClusterUserDefineName string
	KubeConfig            string
	HelmTillerAddr        string
	RemoteShimAddr        string
	K8sClient             oteclient.Interface
	EdgeToClusterChan     chan otev1.ClusterController
	ClusterToEdgeChan     chan otev1.ClusterController
}

// ClusterRegistry defines a data structure to use when a cluster regists.
type ClusterRegistry struct {
	Name           string // uuid
	UserDefineName string
	Listen         string
	Time           int64
	ParentName     string
}

// Serialize is for the ClusterRegistry serialization method.
func (cr *ClusterRegistry) Serialize() ([]byte, error) {
	b, err := json.Marshal(cr)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// ClusterRegistryDeserialize is for the ClusterRegistry deserialize.
func ClusterRegistryDeserialize(b []byte) (*ClusterRegistry, error) {
	cr := ClusterRegistry{}
	err := json.Unmarshal(b, &cr)
	if err != nil {
		return nil, err
	}
	return &cr, nil
}

// WrapperToClusterController is wrapper to ClusterController for ClusterRegistry.
func (cr *ClusterRegistry) WrapperToClusterController(dst string) (*otev1.ClusterController, error) {
	cbyte, err := cr.Serialize()
	if err != nil {
		return nil, fmt.Errorf("serialize clusterregistry(%v) failed: %v", cr, err)
	}
	cc := otev1.ClusterController{
		Spec: otev1.ClusterControllerSpec{
			Destination: dst,
			Body:        string(cbyte),
		},
	}
	return &cc, nil
}

// IsRoot check if clusterName is a root cluster.
func IsRoot(clusterName string) bool {
	return RootClusterName == clusterName
}
