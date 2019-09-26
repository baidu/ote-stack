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

// Package app does all of the work necessary to configure and run k8s_cluster_shim.
package app

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/client-go/informers"
	"k8s.io/klog"

	otev1 "github.com/baidu/ote-stack/pkg/apis/ote/v1"
	"github.com/baidu/ote-stack/pkg/clustershim"
	"github.com/baidu/ote-stack/pkg/clustershim/handler"
	"github.com/baidu/ote-stack/pkg/k8sclient"
	"github.com/baidu/ote-stack/pkg/reporter"
)

var (
	shimSock   string
	kubeConfig string
	helmConfig string
)

const (
	informerDuration = 10 * time.Second
)

// NewK8sClusterShimCommand creates a *cobra.Command object with default parameters.
func NewK8sClusterShimCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "k8s_cluster_shim",
		Short: "k8s_cluster_shim is a component of ote-stack which connect clustercontroller and k8s apiserver",
		Long:  `k8s_cluster_shim is a middleware between clustercontroller and k8s cluster`,
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
			klog.Info("OTE k8s_cluster_shim 1.0")
		},
	}

	cmd.AddCommand(versionCmd)
	cmd.PersistentFlags().StringVarP(&shimSock, "listen", "l",
		":8262", "Websocket address of ClusterShim")
	cmd.PersistentFlags().StringVarP(&kubeConfig, "kube-config", "k", "/root/.kube/config", "KubeConfig file path")
	cmd.PersistentFlags().StringVarP(&helmConfig, "helm-addr", "", "", "Helm proxy address")
	fs := cmd.Flags()
	fs.AddGoFlagSet(flag.CommandLine)

	return cmd
}

// Run runs the k8s cluster shim.
func Run() error {
	// make client to k8s apiserver.
	k8sClient, err := k8sclient.NewK8sClient(kubeConfig)
	if err != nil {
		return err
	}
	klog.Infof("%v", k8sClient)

	signals := make(chan os.Signal, 0)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	s := clustershim.NewShimServer()
	s.RegisterHandler(otev1.ClusterControllerDestAPI, handler.NewK8sHandler(k8sClient))
	// TODO directly connect helm tiller.
	s.RegisterHandler(otev1.ClusterControllerDestHelm, handler.NewHTTPProxyHandler(helmConfig))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reporterContext := &reporter.ReporterContext{
		InformerFactory: informers.NewSharedInformerFactory(k8sClient, informerDuration),
		ClusterName:     s.ClusterName,
		SyncChan:        s.SendChan(),
		StopChan:        ctx.Done(), 
	}

	err = startReporters(reporterContext)
	if err != nil {
		klog.Fatalf("start reporters failed: %v", err)
	}

	go func() {
		<-signals
		s.Close()
		os.Exit(0)
	}()

	if err := s.Serve(shimSock); err != nil {
		klog.Fatalf("can not start shim server: %s ", err.Error())
	}

	return nil
}

func startReporters(ctx *reporter.ReporterContext) error {
	reporters := reporter.NewReporterInitializers()
	for reporterName, initFn := range reporters {
		err := initFn(ctx)
		if err != nil {
			klog.Errorf("init %s reporter failed: %v", reporterName, err)
			return err
		}

		klog.Infof("start reporter %s", reporterName)
	}
	ctx.InformerFactory.Start(ctx.StopChan)
	return nil
}
