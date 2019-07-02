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

// Package k8sclient contains operations of k8s client,
// and operations of k8s crd defined by ote.
package k8sclient

import (
	"fmt"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"

	oteclient "github.com/baidu/ote-stack/pkg/generated/clientset/versioned"
)

// NewClient new a k8s client by k8s config file.
func NewClient(kubeConfig string) (oteclient.Interface, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	loadingRules.ExplicitPath = kubeConfig

	overrides := &clientcmd.ConfigOverrides{ClusterDefaults: clientcmd.ClusterDefaults}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("get kubernetes config from %s failed: %v",
			kubeConfig, err)
	}
	// creates the clientset
	clientset, err := oteclient.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("build client with config from %s failed: %v",
			kubeConfig, err)
	}
	klog.Infof("connect to k8s apiserver %v success", config.Host)
	return clientset, nil
}
