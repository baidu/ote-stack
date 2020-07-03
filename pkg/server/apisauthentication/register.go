package apisauthentication

import (
	"github.com/emicklei/go-restful"

	"github.com/baidu/ote-stack/pkg/server/apisauthentication/resource"
	"github.com/baidu/ote-stack/pkg/server/handler"
	"github.com/baidu/ote-stack/pkg/util"
)

const (
	ServePath = "/apis/authentication.k8s.io/v1"
)

func NewServiceHandler(ctx *handler.HandlerContext) *handler.ServiceHandler {
	serverHandler := map[string]handler.Handler{}

	// TODO add needed server handler here.
	serverHandler[util.ResourceTokenReview] = resource.NewTokenReviewHandler(ctx)

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

	return ws
}
