package app

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/spf13/cobra"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/controller/edgenode"
	"github.com/baidu/ote-stack/pkg/k8sclient"
)

var (
	kubeConfig string
)

func NewEdgeControllerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edgecontroller",
		Short: "edgecontroller is a component using for indicating node status in cloud",
		Long:  "edgecontroller is a component using for indicating node status in cloud",
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
			klog.Info("OTE edgecontroller 1.0")
		},
	}

	cmd.AddCommand(versionCmd)
	cmd.PersistentFlags().StringVarP(&kubeConfig, "kube-config", "k", "/root/.kube/config", "KubeConfig file path")

	fs := cmd.Flags()
	fs.AddGoFlagSet(flag.CommandLine)

	return cmd
}

func Run() error {
	signals := make(chan os.Signal, 0)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	k8sClient, err := k8sclient.NewK8sClient(k8sclient.K8sOption{KubeConfig: kubeConfig})
	if err != nil {
		return fmt.Errorf("get k8s client failed: %v", err)
	}

	oteClient, err := k8sclient.NewClient(kubeConfig)
	if err != nil {
		return fmt.Errorf("get ote client failed: %v", err)
	}

	ctx := &edgenode.ControllerContext{
		OteClient:  oteClient,
		KubeClient: k8sClient,
		StopChan:   make(chan struct{}),
	}

	edgeNodeController := edgenode.NewEdgeNodeController(ctx)
	edgeNodeController.Start()

	go func() {
		<-signals
		klog.Infof("edgecontroller exit")
		os.Exit(0)
	}()

	// hang.
	wait := sync.WaitGroup{}
	wait.Add(1)
	wait.Wait()
	return nil
}
