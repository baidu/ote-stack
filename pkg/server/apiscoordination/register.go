package apiscoordination

import (
	"github.com/emicklei/go-restful"

	"github.com/baidu/ote-stack/pkg/server/apiscoordination/resource"
	"github.com/baidu/ote-stack/pkg/server/handler"
	"github.com/baidu/ote-stack/pkg/util"
)

const (
	ServePath = "/apis/coordination.k8s.io/v1"
)

func NewServiceHandler(ctx *handler.HandlerContext) *handler.ServiceHandler {
	serverHandler := map[string]handler.Handler{}

	// TODO add needed server handler here.
	serverHandler[util.ResourceNodeLease] = resource.NewLeaseHandler(ctx)

	return &handler.ServiceHandler{
		ServerHandler: serverHandler,
		Ctx:           ctx,
	}
}

func NewWebsService(ctx *handler.HandlerContext) *restful.WebService {
	serviceHandler := NewServiceHandler(ctx)

	ws := new(restful.WebService)
	ws.Path(ServePath)
	// Register create handler
	serviceHandler.RegisterCreateHandler(ws)
	serviceHandler.RegisterDeleteHandler(ws)
	serviceHandler.RegisterListHandler(ws)
	serviceHandler.RegisterUpdateHandler(ws)

	return ws
}
