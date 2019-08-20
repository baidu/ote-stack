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

// Package app set flags and command to clustercontroller, and start the program.
package app

import (
	"flag"
	"sync"

	"github.com/spf13/cobra"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/clusterhandler"
	"github.com/baidu/ote-stack/pkg/clustermessage"
	"github.com/baidu/ote-stack/pkg/config"
	"github.com/baidu/ote-stack/pkg/edgehandler"
	"github.com/baidu/ote-stack/pkg/k8sclient"

	oteclient "github.com/baidu/ote-stack/pkg/generated/clientset/versioned"
)

var (
	parentCluster    string
	clusterName      string
	kubeConfig       string
	tunnelListenAddr string
	remoteShimAddr   string
	helmTillerAddr   string
)

// NewClusterControllerCommand creates a *cobra.Command object with default parameters.
func NewClusterControllerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clustercontroller",
		Short: "clustercontroller is a component of ote-stack which manager ote cluster",
		Long: `clustercontroller connects to the others to make a ote cluster,
		which can connect to other cluster like k8s`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := Run(); err != nil {
				klog.Fatal(err)
			}

		},
	}

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Show version",
		Long:  "",
		Run: func(cmd *cobra.Command, args []string) {
			klog.Info("OTE clustercontroller 1.0")
		},
	}

	cmd.AddCommand(versionCmd)
	cmd.PersistentFlags().StringVarP(&parentCluster, "parent-cluster", "p", "", "Cloud tunnel of parent cluster, e.g., 192.168.0.2:8287")
	cmd.PersistentFlags().StringVarP(&clusterName, "cluster-name", "n", config.RootClusterName, "Current cluster name, must be unique")
	cmd.PersistentFlags().StringVarP(&kubeConfig, "kube-config", "k", "/root/.kube/config", "KubeConfig file path")
	cmd.PersistentFlags().StringVarP(&tunnelListenAddr, "tunnel-listen", "l", ":8287", "Cloud tunnel listen address, e.g., 192.168.0.3:8287")
	cmd.PersistentFlags().StringVarP(&remoteShimAddr, "remote-shim-endpoint", "r", "", "remote cluster shim address, e.g., 192.168.0.4:8262")
	cmd.PersistentFlags().StringVarP(&helmTillerAddr, "helm-tiller-addr", "t", "", "helm tiller http proxy addr, e.g., 192.168.0.4:8288")
	fs := cmd.Flags()
	fs.AddGoFlagSet(flag.CommandLine)

	return cmd
}

// Run runs cluster controller.
func Run() error {
	// make client to k8s apiserver if no remote shim available.
	var k8sClient oteclient.Interface
	var err error
	if remoteShimAddr == "" {
		k8sClient, err = k8sclient.NewClient(kubeConfig)
		if err != nil {
			return err
		}
	}
	// make a channel to broadcast to child.
	// and regist edge/cluster handler to the channel.
	edgeToClusterChan := make(chan clustermessage.ClusterMessage)
	// make a channel to return result from cluster handler to edge handler.
	clusterToEdgeChan := make(chan clustermessage.ClusterMessage)
	// make config for cluster controller.
	clusterConfig := &config.ClusterControllerConfig{
		TunnelListenAddr:      tunnelListenAddr,
		ParentCluster:         parentCluster,
		ClusterName:           clusterName,
		ClusterUserDefineName: clusterName,
		K8sClient:             k8sClient,
		HelmTillerAddr:        helmTillerAddr,
		RemoteShimAddr:        remoteShimAddr,
		EdgeToClusterChan:     edgeToClusterChan,
		ClusterToEdgeChan:     clusterToEdgeChan,
	}

	// start edge/cluster handler.
	// connect to parent cluster and regist edge handler to the tunnel.
	edgeHandler := edgehandler.NewEdgeHandler(clusterConfig)
	if err := edgeHandler.Start(); err != nil {
		klog.Fatal(err)
	}
	// listen on tunnel for child.
	clusterHandler, err := clusterhandler.NewClusterHandler(clusterConfig)
	if err != nil {
		klog.Fatal(err)
	}
	if err := clusterHandler.Start(); err != nil {
		klog.Fatal(err)
	}

	// hang.
	wait := sync.WaitGroup{}
	wait.Add(1)
	wait.Wait()
	return nil
}
