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

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"

	oteclient "github.com/baidu/ote-stack/pkg/generated/clientset/versioned"
)

// K8sOption uses for creating a k8s client.
type K8sOption struct {
	// KubeConfig is k8s's config parameter.
	KubeConfig string
	// Burst is the number of request sent to kube-apiserver per second.
	Burst int
	// Qps indicates the maximum qps to the kube-apiserver from this client.
	Qps float32
}

// NewClient new a k8s client by k8s config file.
func NewClient(kubeConfig string) (oteclient.Interface, error) {
	config, err := getRestConfigFromKubeConfigFile(kubeConfig)
	if err != nil {
		return nil, err
	}

	// creates the clientset
	clientset, err := oteclient.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("build client with config from %s failed: %v",
			kubeConfig, err)
	}
	klog.Infof("connect to k8s apiserver %v as ote client success", config.Host)
	return clientset, nil
}

func NewK8sClient(k8sOption K8sOption) (kubernetes.Interface, error) {
	config, err := getRestConfigFromKubeConfigFile(k8sOption.KubeConfig)
	if err != nil {
		return nil, err
	}

	// if burst is not specified, use the default value.
	if k8sOption.Burst != 0 {
		config.Burst = k8sOption.Burst
	}

	// if qps is not specified, use the default value.
	if k8sOption.Qps != 0.0 {
		config.QPS = k8sOption.Qps
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("build client with config from %s failed: %v",
			k8sOption.KubeConfig, err)
	}
	klog.Infof("connect to k8s apiserver %v success", config.Host)
	return clientset, nil
}

func getRestConfigFromKubeConfigFile(kubeConfig string) (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	loadingRules.ExplicitPath = kubeConfig

	overrides := &clientcmd.ConfigOverrides{ClusterDefaults: clientcmd.ClusterDefaults}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("get kubernetes config from %s failed: %v", kubeConfig, err)
	}
	return config, nil
}
