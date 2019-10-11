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

// Package reporter collects edge resource status and reports it to controller manager.
package reporter

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/clustermessage"
)

const (
	ResourceTypeNode = iota + 1
	ResourceTypePod
	ResourceTypeDeployment
	ResourceTypeDaemonset
	ResourceTypeService
	ResourceTypeStatefulset
	ResourceTypeClusterStatus
	ResourceTypeEvent

	ClusterLabel     = "ote-cluster"
	EdgeVersionLabel = "edge-version"
)

//Report defines edge report content.
type Report struct {
	ResourceType int `json:"resourceType"`
	// Body defines different resource status.
	Body []byte `json:"body"`
}

// PodResourceStatus defines pod resource status.
type PodResourceStatus struct {
	// UpdateMap stores created/updated resource obj.
	UpdateMap map[string]*corev1.Pod `json:"updateMap"`
	// DelMap stores deleted resource obj.
	DelMap map[string]*corev1.Pod `json:"delMap"`
	// FullList stores full resource obj.
	FullList []*corev1.Pod `json:"fullList"`
}

// NodeResourceStatus defines node resource status.
type NodeResourceStatus struct {
	// UpdateMap stores created/updated resource obj.
	UpdateMap map[string]*corev1.Node `json:"updateMap"`
	// DelMap stores deleted resource obj.
	DelMap map[string]*corev1.Node `json:"delMap"`
	// FullList stores full resource obj.
	FullList []*corev1.Node `json:"fullList"`
}

//TODO: more resource structure definitions.

// ReporterContext defines the context object for reporter.
type ReporterContext struct {
	// InformerFactory gives access to informers for the reporter.
	InformerFactory informers.SharedInformerFactory
	// ClusterName gets the cluster name.
	ClusterName func() string
	// SyncChan is used for synchronizing status of the edge cluster.
	SyncChan chan clustermessage.ClusterMessage
	// StopChan is the stop channel.
	StopChan <-chan struct{}
}

// InitFunc is used to launch a particular reporter.
type InitFunc func(ctx *ReporterContext) error

// NewReporterInitializers returns a public map of named reporter groups paired to their InitFunc.
func NewReporterInitializers() map[string]InitFunc {
	reporters := map[string]InitFunc{}
	// TODO initialize reporter instance

	reporters["podReporter"] = startPodReporter
	return reporters
}

// IsValid returns the ReporterContext validation result.
func (ctx *ReporterContext) IsValid() bool {
	if ctx == nil {
		klog.Errorf("Failed to create new reporter, ctx is nil")
		return false
	}
	if ctx.InformerFactory == nil {
		klog.Errorf("Failed to create new reporter, InformerFactory is nil")
		return false
	}
	if ctx.SyncChan == nil {
		klog.Errorf("Failed to create new reporter, SyncChan is nil")
		return false
	}
	if ctx.StopChan == nil {
		klog.Errorf("Failed to create new reporter, StopChan is nil")
		return false
	}

	return true
}
