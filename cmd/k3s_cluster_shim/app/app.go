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

// Package app does all of the work necessary to configure and run k3s_cluster_shim.
package app

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"k8s.io/klog"

	otev1 "github.com/baidu/ote-stack/pkg/apis/ote/v1"
	"github.com/baidu/ote-stack/pkg/clustershim"
	"github.com/baidu/ote-stack/pkg/clustershim/handler"
	"github.com/baidu/ote-stack/pkg/k8sclient"
)

var (
	shimSock   string
	kubeConfig string
)

// NewK3sClusterShimCommand creates a *cobra.Command object with default parameters.
func NewK3sClusterShimCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "k3s_cluster_shim",
		Short: "k3s_cluster_shim is a component of ote-stack which connect clustercontroller and k3s apiserver",
		Long:  `k3s_cluster_shim is a middleware between clustercontroller and k3s cluster`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := Run(); err != nil {
				panic(err)
			}

		},
	}

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Show version",
		Long:  "",
		Run: func(cmd *cobra.Command, args []string) {
			klog.Info("OTE k3s_cluster_shim 1.0")
		},
	}

	cmd.AddCommand(versionCmd)
	cmd.PersistentFlags().StringVarP(&shimSock, "listen", "l",
		":8262", "Websocket address of ClusterShim")
	cmd.PersistentFlags().StringVarP(&kubeConfig, "kube-config", "k", "/root/.kube/config", "KubeConfig file path")
	fs := cmd.Flags()
	fs.AddGoFlagSet(flag.CommandLine)

	return cmd
}

// Run runs the k3s cluster shim.
func Run() error {
	// make client to k3s apiserver.
	k3sClient, err := k8sclient.NewK8sClient(k8sclient.K8sOption{KubeConfig: kubeConfig})
	if err != nil {
		return err
	}
	klog.Infof("%v", k3sClient)

	signals := make(chan os.Signal, 0)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	s := clustershim.NewShimServer()
	s.RegisterHandler(otev1.ClusterControllerDestAPI, handler.NewK8sHandler(k3sClient))

	go func() {
		<-signals
		os.Remove(shimSock)
		s.Close()
		os.Exit(0)
	}()

	if err := s.Serve(shimSock); err != nil {
		klog.Errorf("can not start grpc server: %s ", err.Error())
		return err
	}

	return nil
}
