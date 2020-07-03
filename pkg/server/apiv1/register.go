package apiv1

import (
	"github.com/emicklei/go-restful"

	"github.com/baidu/ote-stack/pkg/server/apiv1/resource"
	"github.com/baidu/ote-stack/pkg/server/handler"
	"github.com/baidu/ote-stack/pkg/util"
)

const (
	PathAPIV1 = "/api/v1"
)

func NewServiceHandler(ctx *handler.HandlerContext) *handler.ServiceHandler {
	serverHandler := map[string]handler.Handler{}

	// TODO add needed server handler here.
	serverHandler[util.ResourcePod] = resource.NewPodHandler(ctx)
	serverHandler[util.ResourceNode] = resource.NewNodeHandler(ctx)
	serverHandler[util.ResourceService] = resource.NewServiceHandler(ctx)
	serverHandler[util.ResourceEndpoint] = resource.NewEndpointHandler(ctx)
	serverHandler[util.ResourceConfigMap] = resource.NewConfigMapHandler(ctx)
	serverHandler[util.ResourceSecret] = resource.NewSecretHandler(ctx)
	serverHandler[util.ResourceEvent] = resource.NewEventHandler(ctx)
	serverHandler[util.ResourceNamespace] = resource.NewNamespaceHandler(ctx)

	return &handler.ServiceHandler{
		ServerHandler: serverHandler,
		Ctx:           ctx,
	}
}

func NewWebsService(ctx *handler.HandlerContext) *restful.WebService {
	serviceHandler := NewServiceHandler(ctx)

	ws := new(restful.WebService)
	ws.Path(PathAPIV1)
	// Register create handler
	serviceHandler.RegisterCreateHandler(ws)
	serviceHandler.RegisterDeleteHandler(ws)
	serviceHandler.RegisterListHandler(ws)
	serviceHandler.RegisterUpdateHandler(ws)

	return ws
}
