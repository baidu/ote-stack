// Package app does all of the work necessary to configure and run edgehub
package app

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/k8sclient"
	"github.com/baidu/ote-stack/pkg/loadbalancer"
	"github.com/baidu/ote-stack/pkg/server"
	cert "github.com/baidu/ote-stack/pkg/server/certificate"
	"github.com/baidu/ote-stack/pkg/server/handler"
	"github.com/baidu/ote-stack/pkg/storage"
	"github.com/baidu/ote-stack/pkg/syncer"
)

var (
	kubeConfig   string
	nodeName     string
	dataPath     string
	initServer   string
	lbAddress    string
	synceTimeout int

	serverAddress      string
	serverPort         int
	serverReadTimeout  int
	serverWriteTimeout int
	serverCaFile       string
	serverCertFile     string
	serverKeyFile      string
	tokenFile          string

	// use for edgehub start
	edgeStore  *storage.EdgehubStorage
	k8sClient  kubernetes.Interface
	edgeLB     *loadbalancer.LoadBalancer
	syncerCtx  *syncer.SyncContext
	lbStopChan chan struct{}
)

func NewEdgehubCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edgehub",
		Short: "edgehub is a component using for marginal autonomy",
		Long:  "edgehub is a proxy for master to interact with node",
		Run: func(cmd *cobra.Command, args []string) {
		},
	}

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Show version",
		Long:  "",
		Run: func(cmd *cobra.Command, args []string) {
			klog.Info("OTE edgehub 1.0")
		},
	}

	k8sCmd := &cobra.Command{
		Use:   "k8s",
		Short: "edgehub uses for k8s cluster",
		Long:  "",
		Run: func(cmd *cobra.Command, args []string) {
			if err := k8sRun(); err != nil {
				panic(err)
			}
		},
	}

	k3sCmd := &cobra.Command{
		Use:   "k3s",
		Short: "edgehub uses for k3s cluster",
		Long:  "",
		Run: func(cmd *cobra.Command, args []string) {
			if err := k3sRun(); err != nil {
				panic(err)
			}
		},
	}

	cmd.AddCommand(versionCmd)
	cmd.AddCommand(k8sCmd)
	cmd.AddCommand(k3sCmd)

	k8sCmd.PersistentFlags().StringVarP(&kubeConfig, "kube-config", "k", "/root/.kube/config", "KubeConfig file path")
	k8sCmd.PersistentFlags().StringVarP(&nodeName, "node-name", "n", "", "Edge Node's name")
	k8sCmd.PersistentFlags().StringVarP(&dataPath, "data-path", "d", "./data", "data's directory")
	k8sCmd.PersistentFlags().IntVar(&synceTimeout, "sync-timeout", 60, "syncer sync cache timeout")
	k8sCmd.PersistentFlags().StringVarP(&initServer, "init-server", "s", "127.0.0.1:8080", "init backend server's address")
	k8sCmd.PersistentFlags().StringVarP(&lbAddress, "lb-address", "l", "127.0.0.1:6888", "load balancer's access address")
	k8sCmd.PersistentFlags().StringVar(&serverAddress, "server-address", "127.0.0.1", "Binding ip by edge server")
	k8sCmd.PersistentFlags().IntVar(&serverPort, "server-port", 8080, "Binding port by edge server")
	k8sCmd.PersistentFlags().IntVar(&serverReadTimeout, "server-read-timeout", 5, "Edge server read timeout")
	k8sCmd.PersistentFlags().IntVar(&serverWriteTimeout, "server-write-timeout", 5, "Edge server write timeout")
	k8sCmd.PersistentFlags().StringVar(&serverCaFile, "server-ca-file", "./ssl/ca.pem", "Edge server tls ca file")
	k8sCmd.PersistentFlags().StringVar(&serverCertFile, "server-cert-file", "./ssl/edge-server.pem", "Edge server tls cert file")
	k8sCmd.PersistentFlags().StringVar(&serverKeyFile, "server-key-file", "./ssl/edge-server-key.pem", "Edge server tls key file")

	k3sCmd.PersistentFlags().StringVarP(&kubeConfig, "kube-config", "k", "/root/.kube/config", "KubeConfig file path")
	k3sCmd.PersistentFlags().StringVarP(&nodeName, "node-name", "n", "", "Edge Node's name")
	k3sCmd.PersistentFlags().StringVarP(&dataPath, "data-path", "d", "./data", "data's directory")
	k3sCmd.PersistentFlags().IntVar(&synceTimeout, "sync-timeout", 60, "syncer sync cache timeout")
	k3sCmd.PersistentFlags().StringVarP(&initServer, "init-server", "s", "127.0.0.1:8080", "init backend server's address")
	k3sCmd.PersistentFlags().StringVarP(&lbAddress, "lb-address", "l", "127.0.0.1:6888", "load balancer's access address")
	k3sCmd.PersistentFlags().StringVar(&serverAddress, "server-address", "127.0.0.1", "Binding ip by edge server")
	k3sCmd.PersistentFlags().IntVar(&serverPort, "server-port", 8080, "Binding port by edge server")
	k3sCmd.PersistentFlags().StringVar(&serverCertFile, "server-cert-file", "./ssl/edge-server.pem", "Edge server tls cert file")
	k3sCmd.PersistentFlags().StringVar(&serverKeyFile, "server-key-file", "./ssl/edge-server-key.pem", "Edge server tls key file")
	k3sCmd.PersistentFlags().StringVar(&tokenFile, "node-token-file", "./ssl/token", "Node token file")

	fs := k8sCmd.Flags()
	fs.AddGoFlagSet(flag.CommandLine)

	fs = k3sCmd.Flags()
	fs.AddGoFlagSet(flag.CommandLine)

	return cmd
}

func k8sRun() error {
	if err := initEdgehub(); err != nil {
		return err
	}

	signals := make(chan os.Signal, 0)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	// edge server ctx
	serverCtx := &server.ServerContext{
		BindAddr:     serverAddress,
		BindPort:     serverPort,
		ReadTimeout:  serverReadTimeout,
		WriteTimeout: serverWriteTimeout,
		CaFile:       serverCaFile,
		CertFile:     serverCertFile,
		KeyFile:      serverKeyFile,
		StopChan:     make(chan bool, 0),

		HandlerCtx: &handler.HandlerContext{
			Store:          edgeStore,
			Lb:             edgeLB,
			K8sClient:      k8sClient,
			EdgeSubscriber: syncer.GetSubscriber(),
		},
	}

	go stopEdgehub(signals, serverCtx)

	// start server
	k8sServer := server.NewEdgeK8sServer(serverCtx)
	if err := k8sServer.StartServer(serverCtx); err != nil {
		klog.Errorf("start edge server error:%v", err)
		os.Exit(1)
	}

	klog.Info("edgehub exit")
	return nil
}

func k3sRun() error {
	if err := initEdgehub(); err != nil {
		return err
	}

	signals := make(chan os.Signal, 0)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	// edge server ctx
	serverCtx := &server.ServerContext{
		BindAddr: serverAddress,
		BindPort: serverPort,
		CertFile: serverCertFile,
		KeyFile:  serverKeyFile,
		StopChan: make(chan bool, 0),

		HandlerCtx: &handler.HandlerContext{
			Store:          edgeStore,
			Lb:             edgeLB,
			K8sClient:      k8sClient,
			EdgeSubscriber: syncer.GetSubscriber(),
		},
		CertCtx: &cert.CertContext{
			DataPath:  dataPath,
			CaFile:    serverCertFile,
			TokenFile: tokenFile,
			ServerURL: "https://" + lbAddress,
			Store:     edgeStore,
			Lb:        edgeLB,
			K8sClient: k8sClient,
		},
	}

	go stopEdgehub(signals, serverCtx)

	// start server
	k3sServer := server.NewEdgeK3sServer(serverCtx)
	if err := k3sServer.StartServer(serverCtx); err != nil {
		klog.Errorf("start edge server error:%v", err)
		os.Exit(1)
	}

	klog.Infof("edgehub exit")
	return nil
}

func initEdgehub() error {
	var err error

	if !isEdgehubReady() {
		return fmt.Errorf("start edgehub failed")
	}

	// init k8s client
	k8sClient, err = k8sclient.NewK8sClient(k8sclient.K8sOption{KubeConfig: kubeConfig})
	if err != nil {
		return fmt.Errorf("get k8s client failed: %v", err)
	}

	// Init storage
	config := &storage.Config{
		Path: dataPath + "/db",
	}
	edgeStore, err = storage.NewEdgehubStore(config)
	if err != nil {
		return fmt.Errorf("Init storage failed: %v", err)
	}

	// init lb config
	lbStopChan = make(chan struct{})

	lbConfig := &loadbalancer.Config{
		KubeClient:    k8sClient,
		ServerAddress: initServer,
		LbAddress:     lbAddress,
		HealthChan:    make(chan bool),
		StopChan:      lbStopChan,
	}

	// init subscriber
	syncer.InitSubscriber()

	// syncer ctx
	syncerCtx = &syncer.SyncContext{
		NodeName:    nodeName,
		KubeClient:  k8sClient,
		Store:       edgeStore,
		SyncTimeout: synceTimeout,
	}

	// start syncer
	go func() {
		for {
			select {
			case isRemoteEnable := <-lbConfig.HealthChan:
				if isRemoteEnable {
					edgeStore.RemoteEnable = true

					if err := syncer.StartSyncer(syncerCtx); err != nil {
						klog.Errorf("StartSyncer failed: %v", err)
						syncer.StopSyncer(syncerCtx)
						continue
					}
				} else {
					if err := syncer.StopSyncer(syncerCtx); err != nil {
						klog.Errorf("StopSyncer failed: %v", err)
						continue
					}

					edgeStore.RemoteEnable = false
				}
			}
		}
	}()

	// start load balancer
	edgeLB, err = loadbalancer.Start(lbConfig)
	if err != nil {
		klog.Fatal(err)
	}

	return nil
}

func stopEdgehub(signals chan os.Signal, serverCtx *server.ServerContext) {
	<-signals
	serverCtx.StopChan <- true
	syncer.StopSyncer(syncerCtx)
	close(lbStopChan)
	edgeStore.Close()
}

// isEdgehubReady check if edgehub's params needed is specified.
func isEdgehubReady() bool {
	if kubeConfig == "" {
		klog.Errorf("kubeconfig is not specified")
		return false
	}
	if nodeName == "" {
		name, err := os.Hostname()
		if err != nil {
			klog.Errorf("node name is not specified")
			return false
		}

		nodeName = name
	}

	return true
}
