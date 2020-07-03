package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/baidu/ote-stack/pkg/loadbalancer"
	"github.com/baidu/ote-stack/pkg/server/handler"
	"github.com/baidu/ote-stack/pkg/storage"
	"github.com/baidu/ote-stack/pkg/syncer"
)

func TestCheckK8sValid(t *testing.T) {
	ctx := &ServerContext{}
	server := NewEdgeK8sServer(ctx)

	err := server.CheckValid(ctx)
	assert.NotNil(t, err)

	ctx.BindAddr = "127.0.0.1"
	ctx.BindPort = 8080
	ctx.CaFile = "."
	ctx.CertFile = "."
	ctx.KeyFile = "."
	ctx.StopChan = make(chan bool)

	handlerCtx := &handler.HandlerContext{
		Store:          &storage.EdgehubStorage{},
		K8sClient:      &k8sfake.Clientset{},
		Lb:             &loadbalancer.LoadBalancer{},
		EdgeSubscriber: syncer.GetSubscriber(),
	}

	ctx.HandlerCtx = handlerCtx
	err = server.CheckValid(ctx)
	assert.Nil(t, err)
}
