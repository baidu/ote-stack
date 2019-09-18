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
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	"github.com/baidu/ote-stack/pkg/clustermessage"
	oteclient "github.com/baidu/ote-stack/pkg/generated/clientset/versioned"
	oteinformer "github.com/baidu/ote-stack/pkg/generated/informers/externalversions"
	"github.com/baidu/ote-stack/pkg/tunnel"
)

var (
	// TODO add controllers here
	Controllers = map[string]InitFunc{}
)

// ControllerContext is the context needed by all controllers.
// ControllerContext woubld be a param when start a controller.
type ControllerContext struct {
	K8sContext

	// a channel to publish msg to root cluster controller
	PublishChan chan clustermessage.ClusterMessage
	// a tunnel connected to root cluster controller
	controllerTunnel tunnel.ControllerTunnel
}

// InitFunc is the function to start a controller within a context.
type InitFunc func(ctx *ControllerContext) error

// K8sContext is the context of all object related to k8s.
type K8sContext struct {
	OteClient          oteclient.Interface
	OteInformerFactory oteinformer.SharedInformerFactory

	K8sClient       kubernetes.Interface
	InformerFactory informers.SharedInformerFactory
}
