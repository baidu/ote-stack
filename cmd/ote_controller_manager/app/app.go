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

// Package app set flags and command to ote_controller_manager, and start the program.
package app

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/controllermanager"
	oteclient "github.com/baidu/ote-stack/pkg/generated/clientset/versioned"
	oteinformer "github.com/baidu/ote-stack/pkg/generated/informers/externalversions"
	"github.com/baidu/ote-stack/pkg/k8sclient"
	"github.com/baidu/ote-stack/pkg/tunnel"
)

const (
	informerDuration = 10 * time.Second
	leaseDuration    = 15 * time.Second
	renewDeadline    = 10 * time.Second
	retryPeriod      = 2 * time.Second

	oteControllerManagerName = "ote-controller-manager"
)

var (
	kubeConfig                string
	rootClusterControllerAddr string
)

// NewOTEControllerManagerCommand creates a *cobra.Command object with default parameters.
func NewOTEControllerManagerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ote_controller_manager",
		Short: "ote_controller_manager is a component of ote-stack which manager controllers of multi cluster",
		Long: `ote_controller_manager connects to root clustercontroller,
		publish task and write multi cluster info back to center storage`,
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
			klog.Info("OTE ote_controller_manager 1.0")
		},
	}

	cmd.AddCommand(versionCmd)
	cmd.PersistentFlags().StringVarP(&kubeConfig, "kube-config", "k",
		"/root/.kube/config", "KubeConfig file path")
	cmd.PersistentFlags().StringVarP(&rootClusterControllerAddr, "root-cluster-controller", "r",
		":8272",
		"root clustercontroller address, could be a front load balancer, e.g., 192.168.0.4:8272")
	fs := cmd.Flags()
	fs.AddGoFlagSet(flag.CommandLine)

	return cmd
}

// Run runs ote_controller_manager.
func Run() error {
	// make client to k8s apiserver.
	oteClient, err := k8sclient.NewClient(kubeConfig)
	if err != nil {
		return err
	}
	k8sClient, err := k8sclient.NewK8sClient(kubeConfig)
	if err != nil {
		return err
	}

	// connect to root clustercontroller
	controllerTunnel := tunnel.NewControllerTunnel(rootClusterControllerAddr)
	err = controllerTunnel.Start()
	if err != nil {
		return err
	}
	run := func(c context.Context) {
		ctx := createControllerContext(oteClient, k8sClient)
		// start all controllers
		ctx.PublishChan = controllerTunnel.SendChan()
		err = startControllers(ctx)
		if err != nil {
			klog.Fatalf("start controllers failed: %v", err)
		}
	}

	// leader elect
	id, err := os.Hostname()
	if err != nil {
		return err
	}

	// add a uniquifier so that two processes on the same host don't accidentally both become active
	id = id + "_" + string(uuid.NewUUID())
	rl, err := resourcelock.New(
		//resourcelock.EndpointsResourceLock,
		resourcelock.LeasesResourceLock,
		"kube-system",
		oteControllerManagerName,
		k8sClient.CoreV1(),
		k8sClient.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity: id,
			// TODO add event recorder for debug
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
			OnStartedLeading: run,
			OnStoppedLeading: func() {
				klog.Fatalf("leaderelection lost")
			},
		},
		// TODO add watch dog
		// participate leader-election if it is connected to cluster controller
		Name: oteControllerManagerName,
	})

	// hang.
	wait := sync.WaitGroup{}
	wait.Add(1)
	wait.Wait()
	return nil
}

func createControllerContext(oteClient oteclient.Interface,
	k8sClient kubernetes.Interface) *controllermanager.ControllerContext {
	oteSharedInformers := oteinformer.NewSharedInformerFactory(oteClient, informerDuration)
	sharedInformers := informers.NewSharedInformerFactory(k8sClient, informerDuration)
	return &controllermanager.ControllerContext{
		OteClient:          oteClient,
		OteInformerFactory: oteSharedInformers,
		K8sClient:          k8sClient,
		InformerFactory:    sharedInformers,
	}
}

func startControllers(ctx *controllermanager.ControllerContext) error {
	for controllerName, initFn := range controllermanager.Controllers {
		err := initFn(ctx)
		if err != nil {
			klog.Errorf("init %s controller failed: %v", controllerName, err)
			return err
		}

		klog.Infof("start controller %s", controllerName)
	}
	return nil
}
