package resource

import (
	"net/http"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/server/handler"
	"github.com/baidu/ote-stack/pkg/util"
)

type SubjectAccessReviewHandler struct {
	ctx *handler.HandlerContext
}

func NewSubjectAccessReviewHandler(ctx *handler.HandlerContext) handler.Handler {
	return &SubjectAccessReviewHandler{
		ctx: ctx,
	}
}

func (sh *SubjectAccessReviewHandler) Create(body []byte, uri string) *handler.Response {
	sar, err := util.GetObjectFromSerializeData(util.ResourceSubjectAccessReview, body)
	if err != nil {
		klog.Errorf("get object from serialized data failed: %v", err)
		return &handler.Response{
			Code: http.StatusBadRequest,
			Err:  err,
		}
	}

	key, err := cache.MetaNamespaceKeyFunc(sar)
	if err != nil {
		klog.Errorf("get subjectaccessreview key failed: %v", err)
		return &handler.Response{
			Code: http.StatusBadRequest,
			Err:  err,
		}
	}

	return handler.DoCreate(sh.ctx, util.ResourceSubjectAccessReview, key, uri, body)
}

func (sh *SubjectAccessReviewHandler) Delete(key, uri string, body []byte) *handler.Response {
	return handler.DoDelete(sh.ctx, util.ResourceSubjectAccessReview, key, uri, body)
}

func (sh *SubjectAccessReviewHandler) List(name, namespace, uri string) *handler.Response {
	return &handler.Response{Code: http.StatusOK}
}

func (sh *SubjectAccessReviewHandler) Get(key string) *handler.Response {
	return handler.DoGet(sh.ctx, util.ResourceSubjectAccessReview, key)
}

func (sh *SubjectAccessReviewHandler) UpdatePut(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePut(sh.ctx, util.ResourceSubjectAccessReview, key, uri, body)
}

func (sh *SubjectAccessReviewHandler) UpdatePatch(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePatch(sh.ctx, util.ResourceSubjectAccessReview, key, uri, body)
}

func (sh *SubjectAccessReviewHandler) GetInitWatchEvent(fieldSelector, namespace, name string) []metav1.WatchEvent {
	return []metav1.WatchEvent{}
}
