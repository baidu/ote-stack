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

package reporter

import (
	"k8s.io/client-go/informers"

	"github.com/baidu/ote-stack/pkg/clustermessage"
)

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

// NewReporterInitializers returns a public map of named reporter groups
// paired to their InitFunc.
func NewReporterInitializers() map[string]InitFunc {
	reporters := map[string]InitFunc{}
	// TODO initialize reporter instance
	// reporters["nodeReporter"] = startNodeReporter
	return reporters
}
