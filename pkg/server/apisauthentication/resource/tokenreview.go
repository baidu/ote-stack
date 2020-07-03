package resource

import (
	"net/http"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/server/handler"
	"github.com/baidu/ote-stack/pkg/util"
)

type TokenReviewHandler struct {
	ctx *handler.HandlerContext
}

func NewTokenReviewHandler(ctx *handler.HandlerContext) handler.Handler {
	return &TokenReviewHandler{
		ctx: ctx,
	}
}

func (th *TokenReviewHandler) Create(body []byte, uri string) *handler.Response {
	tokenreview, err := util.GetObjectFromSerializeData(util.ResourceTokenReview, body)
	if err != nil {
		klog.Errorf("get object from serialized data failed: %v", err)
		return &handler.Response{
			Code: http.StatusBadRequest,
			Err:  err,
		}
	}

	key, err := cache.MetaNamespaceKeyFunc(tokenreview)
	if err != nil {
		klog.Errorf("get tokenreview key failed: %v", err)
		return &handler.Response{
			Code: http.StatusBadRequest,
			Err:  err,
		}
	}

	return handler.DoCreate(th.ctx, util.ResourceTokenReview, key, uri, body)
}

func (th *TokenReviewHandler) Delete(key, uri string, body []byte) *handler.Response {
	return handler.DoDelete(th.ctx, util.ResourceTokenReview, key, uri, body)
}

func (th *TokenReviewHandler) List(name, namespace, uri string) *handler.Response {
	return &handler.Response{Code: http.StatusInternalServerError}
}

func (th *TokenReviewHandler) Get(key string) *handler.Response {
	return handler.DoGet(th.ctx, util.ResourceTokenReview, key)
}

func (th *TokenReviewHandler) UpdatePut(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePut(th.ctx, util.ResourceTokenReview, key, uri, body)
}

func (th *TokenReviewHandler) UpdatePatch(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePatch(th.ctx, util.ResourceTokenReview, key, uri, body)
}

func (th *TokenReviewHandler) GetInitWatchEvent(fieldSelector, namespace, name string) []metav1.WatchEvent {
	return []metav1.WatchEvent{}
}
