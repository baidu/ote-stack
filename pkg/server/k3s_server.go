package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"path/filepath"

	"github.com/emicklei/go-restful"
	"github.com/rancher/dynamiclistener"
	"github.com/rancher/dynamiclistener/factory"
	"github.com/rancher/dynamiclistener/storage/file"
	"github.com/rancher/dynamiclistener/storage/memory"
	"k8s.io/klog"

	auth "github.com/baidu/ote-stack/pkg/server/apisauthentication"
	authr "github.com/baidu/ote-stack/pkg/server/apisauthorization"
	acd "github.com/baidu/ote-stack/pkg/server/apiscoordination"
	anw "github.com/baidu/ote-stack/pkg/server/apisnetworking"
	an "github.com/baidu/ote-stack/pkg/server/apisnode"
	as "github.com/baidu/ote-stack/pkg/server/apisstorage"
	"github.com/baidu/ote-stack/pkg/server/apiv1"
	crt "github.com/baidu/ote-stack/pkg/server/certificate"
	"github.com/baidu/ote-stack/pkg/server/handler"
)

type edgeK3sServer struct {
	ctx *ServerContext
}

func NewEdgeK3sServer(ctx *ServerContext) EdgeServer {
	return &edgeK3sServer{
		ctx: ctx,
	}
}

func newListener(serverCtx *ServerContext) (net.Listener, http.Handler, error) {
	tcp, err := dynamiclistener.NewTCPListener(serverCtx.BindAddr, serverCtx.BindPort)
	if err != nil {
		return nil, nil, err
	}

	cert, key, err := factory.LoadCerts(serverCtx.CertFile, serverCtx.KeyFile)
	if err != nil {
		return nil, nil, err
	}

	storage := tlsStorage(serverCtx.CertCtx.DataPath)
	return dynamiclistener.NewListener(tcp, storage, cert, key, dynamiclistener.Config{
		CN:           "edgehub",
		Organization: []string{"edgehub"},
		TLSConfig: tls.Config{
			ClientAuth: tls.RequestClientCert,
		},
	})
}

func tlsStorage(dataDir string) dynamiclistener.TLSStorage {
	fileStorage := file.New(filepath.Join(dataDir, "dynamic-cert.json"))
	return memory.NewBacked(fileStorage)
}

func startClusterAndHTTPS(ctx context.Context, serverCtx *ServerContext) error {
	l, handler, err := newListener(serverCtx)
	if err != nil {
		klog.Errorf("new listener failed: %v", err)
		return err
	}

	handler, err = getHandler(handler, serverCtx.HandlerCtx, serverCtx.CertCtx)
	if err != nil {
		klog.Errorf("get handler failed: %v", err)
		return err
	}

	server := http.Server{
		Handler: handler,
	}

	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()

	if err := server.Serve(l); err != nil {
		return err
	}

	return nil
}

func getHandler(handler http.Handler, ctx *handler.HandlerContext, certCtx *crt.CertContext) (http.Handler, error) {
	// TODO add need http handler here.
	wsContainer := restful.NewContainer()
	wsContainer.Add(crt.NewAuthWebService(certCtx))
	wsContainer.Add(apiv1.NewWebsService(ctx))
	wsContainer.Add(anw.NewWebsService(ctx))
	wsContainer.Add(as.NewWebsService(ctx))
	wsContainer.Add(acd.NewWebsService(ctx))
	wsContainer.Add(an.NewWebsService(ctx))
	wsContainer.Add(auth.NewWebsService(ctx))
	wsContainer.Add(authr.NewWebsService(ctx))

	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		handler.ServeHTTP(rw, req)
		wsContainer.ServeHTTP(rw, req)
	}), nil
}

func (e *edgeK3sServer) StartServer(ctx *ServerContext) error {
	if err := e.CheckValid(ctx); err != nil {
		return err
	}

	newCtx, cancel := context.WithCancel(context.Background())

	go func() {
		<-ctx.StopChan
		cancel()
	}()

	klog.Info("EdgeServer starting...")

	if err := startClusterAndHTTPS(newCtx, ctx); err != nil {
		if err == http.ErrServerClosed {
			klog.Info("EdgeServer stopped.")
		} else {
			return err
		}
	}

	return nil
}

func (e *edgeK3sServer) CheckValid(ctx *ServerContext) error {
	if ctx == nil {
		return fmt.Errorf("Server context is nil")
	}

	if net.ParseIP(ctx.BindAddr) == nil {
		return fmt.Errorf("Server bind address %s is an invalid IP.", ctx.BindAddr)
	}

	if ctx.BindPort < 0 || ctx.BindPort > 65535 {
		return fmt.Errorf("Server bind port %d is out of range 0-65535", ctx.BindPort)
	}

	if ctx.CertFile == "" {
		return fmt.Errorf("no specify the server tls cert file")
	}

	if ctx.KeyFile == "" {
		return fmt.Errorf("no specify the server tls key file")
	}

	if ctx.StopChan == nil {
		return fmt.Errorf("stop channel for server is nil")
	}

	if ctx.HandlerCtx == nil || !ctx.HandlerCtx.IsValid() {
		return fmt.Errorf("server handleCtx is invalid")
	}

	if ctx.CertCtx == nil || !ctx.CertCtx.IsValid() {
		return fmt.Errorf("server certCtx is invalid")
	}

	return nil
}
