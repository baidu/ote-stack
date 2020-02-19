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
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/clusterhandler"
	"github.com/baidu/ote-stack/pkg/clustermessage"
	"github.com/baidu/ote-stack/pkg/config"
	"github.com/baidu/ote-stack/pkg/edgehandler"
	"github.com/baidu/ote-stack/pkg/eventrecorder"
	oteclient "github.com/baidu/ote-stack/pkg/generated/clientset/versioned"
	"github.com/baidu/ote-stack/pkg/k8sclient"
)

const (
	leaseDuration = 15 * time.Second
	renewDeadline = 10 * time.Second
	retryPeriod   = 2 * time.Second

	oteRootClusterControllerName = "ote-root-cluster-controller"

	RootClusterToEdgeChanBuffer = 10000
	RootEdgeToClusterChanBuffer = 10000
)

var (
	parentCluster    string
	clusterName      string
	kubeConfig       string
	tunnelListenAddr string
	remoteShimAddr   string
	helmTillerAddr   string
	leaderElection   bool
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
	cmd.PersistentFlags().BoolVarP(&leaderElection, "leader-election", "e", false, "leader elect if this is the root")
	fs := cmd.Flags()
	fs.AddGoFlagSet(flag.CommandLine)

	return cmd
}

// Run runs cluster controller.
func Run() error {
	// make client to k8s apiserver if no remote shim available.
	var oteK8sClient oteclient.Interface
	var err error
	if remoteShimAddr == "" || config.IsRoot(clusterName) {
		klog.Infof("init k8s client")
		oteK8sClient, err = k8sclient.NewClient(kubeConfig)
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
		LeaderListenAddr:      "",
		ParentCluster:         parentCluster,
		ClusterName:           clusterName,
		ClusterUserDefineName: clusterName,
		K8sClient:             oteK8sClient,
		HelmTillerAddr:        helmTillerAddr,
		RemoteShimAddr:        remoteShimAddr,
		EdgeToClusterChan:     edgeToClusterChan,
		ClusterToEdgeChan:     clusterToEdgeChan,
	}

	// if root cc connects to shim, it should use two channel to transfer message.
	if config.IsRoot(clusterName) && remoteShimAddr != "" {
		// make a channel for root cc's edge handler reporting message to cluster handler.
		rootEdgeToClusterChan := make(chan *clustermessage.ClusterMessage, RootEdgeToClusterChanBuffer)
		// make a channel for root cc's cluster handler returning message to edge handler.
		rootClusterToEdgeChan := make(chan *clustermessage.ClusterMessage, RootClusterToEdgeChanBuffer)

		clusterConfig.RootEdgeToClusterChan = rootEdgeToClusterChan
		clusterConfig.RootClusterToEdgeChan = rootClusterToEdgeChan
	}

	// listen on tunnel for child.
	clusterHandler, err := clusterhandler.NewClusterHandler(clusterConfig)
	if err != nil {
		klog.Fatal(err)
	}

	// if this cc should participate in leader election, start the cluster handler when become the leader
	if leaderElection && config.IsRoot(clusterName) {
		k8sClient, err := k8sclient.NewK8sClient(k8sclient.K8sOption{KubeConfig: kubeConfig})
		if err != nil {
			return err
		}
		// leader elect if this is the root
		id, err := os.Hostname()
		if err != nil {
			return err
		}

		// add a uniquifier so that two processes on the same host don't accidentally both become active
		leaderAddrSep := byte('_')
		id = id + string(leaderAddrSep) + tunnelListenAddr
		rl, err := resourcelock.New(
			resourcelock.EndpointsResourceLock,
			"kube-system",
			oteRootClusterControllerName,
			k8sClient.CoreV1(),
			resourcelock.ResourceLockConfig{
				Identity: id,
				// add event recorder for debug
				EventRecorder: &eventrecorder.LocalEventRecorder{},
			})
		if err != nil {
			return fmt.Errorf("error creating lock: %v", err)
		}

		leaderelection.RunOrDie(context.TODO(), leaderelection.LeaderElectionConfig{
			Lock:          rl,
			LeaseDuration: leaseDuration,
			RenewDeadline: renewDeadline,
			RetryPeriod:   retryPeriod,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: func(c context.Context) {
					setLeaderListenAddr(clusterConfig, "", tunnelListenAddr)
				},
				OnStoppedLeading: func() {
					klog.Fatalf("leaderelection lost")
				},
				OnNewLeader: func(identify string) {
					// get listen addr of leader
					leaderAddr := identify[strings.LastIndexByte(identify, leaderAddrSep)+1:]
					klog.Infof("leader listen on %s", leaderAddr)
					setLeaderListenAddr(clusterConfig, leaderAddr, tunnelListenAddr)
					if err := clusterHandler.Start(); err != nil {
						klog.Fatal(err)
					}
				},
			},
			// TODO add watch dog
			// participate leader-election if it is connected to cluster controller
			Name: oteRootClusterControllerName,
		})
	} else {
		if err := clusterHandler.Start(); err != nil {
			klog.Fatal(err)
		}
	}

	// start edge/cluster handler.
	// connect to parent cluster and regist edge handler to the tunnel.
	edgeHandler := edgehandler.NewEdgeHandler(clusterConfig)
	if err := edgeHandler.Start(); err != nil {
		klog.Fatal(err)
	}

	// hang.
	wait := sync.WaitGroup{}
	wait.Add(1)
	wait.Wait()
	return nil
}

func setLeaderListenAddr(c *config.ClusterControllerConfig, leaderAddr, currentAddr string) {
	if leaderAddr == currentAddr {
		return
	}
	c.LeaderListenAddr = leaderAddr
}
