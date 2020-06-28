package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/emicklei/go-restful"
	"k8s.io/klog"

	csr "github.com/baidu/ote-stack/pkg/server/apiscertificates"
	"github.com/baidu/ote-stack/pkg/server/apiv1"
	certutil "k8s.io/client-go/util/cert"
)

const (
	ServerShowdownTimeout = 15
)

type edgeK8sServer struct {
	ctx *ServerContext
}

func NewEdgeK8sServer(ctx *ServerContext) EdgeServer {
	return &edgeK8sServer{
		ctx: ctx,
	}
}

func newHealthWs() *restful.WebService {
	ws := new(restful.WebService)
	ws.Route(ws.Path("/healthz").GET("").To(
		func(request *restful.Request, response *restful.Response) {
			response.Write([]byte("ok"))
		}))
	return ws
}

func (e *edgeK8sServer) CheckValid(ctx *ServerContext) error {
	if ctx == nil {
		return fmt.Errorf("Server context is nil")
	}

	if net.ParseIP(ctx.BindAddr) == nil {
		return fmt.Errorf("Server bind address %s is an invalid IP.", ctx.BindAddr)
	}

	if ctx.BindPort < 0 || ctx.BindPort > 65535 {
		return fmt.Errorf("Server bind port %d is out of range 0-65535", ctx.BindPort)
	}

	if ctx.CaFile == "" {
		return fmt.Errorf("no specify the server tls ca file")
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
		return fmt.Errorf("no specify the server handleCtx")
	}

	return nil
}

func (e *edgeK8sServer) StartServer(ctx *ServerContext) error {
	if err := e.CheckValid(ctx); err != nil {
		return err
	}

	tlsCert, err := tls.LoadX509KeyPair(ctx.CertFile, ctx.KeyFile)
	if err != nil {
		return fmt.Errorf("unable to load server certificate: %v", err)
	}

	clientCAs, err := certutil.CertsFromFile(ctx.CaFile)
	certPool := x509.NewCertPool()
	for _, cert := range clientCAs {
		certPool.AddCert(cert)
	}

	// TODO add need http handler here.
	wsContainer := restful.NewContainer()
	wsContainer.Add(apiv1.NewWebsService(ctx.HandlerCtx))
	wsContainer.Add(newHealthWs())
	wsContainer.Add(csr.NewWebsService(ctx.HandlerCtx))

	bindAddr := fmt.Sprintf("%s:%d", ctx.BindAddr, ctx.BindPort)

	edgeServer := &http.Server{
		Addr:         bindAddr,
		Handler:      wsContainer,
		ReadTimeout:  time.Duration(ctx.ReadTimeout) * time.Minute,
		WriteTimeout: time.Duration(ctx.WriteTimeout) * time.Minute,
		TLSConfig: &tls.Config{
			NameToCertificate: make(map[string]*tls.Certificate),
			Certificates:      []tls.Certificate{tlsCert},
			MinVersion:        tls.VersionTLS12,
			ClientAuth:        tls.RequestClientCert,
			ClientCAs:         certPool,
			NextProtos:        []string{"h2", "http/1.1"},
		},
	}

	go func() {
		<-ctx.StopChan

		ctx, cancel := context.WithTimeout(context.Background(), ServerShowdownTimeout*time.Second)
		defer cancel()

		if err := edgeServer.Shutdown(ctx); err != nil {
			klog.Errorf("Shutdown EdgeServer error, %v", err)
		}
	}()

	klog.Info("EdgeServer starting...")

	if err := edgeServer.ListenAndServeTLS(ctx.CertFile, ctx.KeyFile); err != nil {
		if err == http.ErrServerClosed {
			klog.Info("EdgeServer stopped.")
		} else {
			return err
		}
	}

	return nil
}
