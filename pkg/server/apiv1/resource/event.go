package resource

import (
	"net/http"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/server/handler"
	"github.com/baidu/ote-stack/pkg/util"
)

type EventHandler struct {
	ctx *handler.HandlerContext
}

func NewEventHandler(ctx *handler.HandlerContext) handler.Handler {
	return &EventHandler{
		ctx: ctx,
	}
}

func (eh *EventHandler) Create(body []byte, uri string) *handler.Response {
	event, err := util.GetObjectFromSerializeData(util.ResourceEvent, body)
	if err != nil {
		klog.Errorf("get object from serialized data failed: %v", err)
		return &handler.Response{
			Code: http.StatusBadRequest,
			Err:  err,
		}
	}

	key, err := cache.MetaNamespaceKeyFunc(event)
	if err != nil {
		klog.Errorf("get event key failed: %v", err)
		return &handler.Response{
			Code: http.StatusBadRequest,
			Err:  err,
		}
	}

	return handler.DoCreate(eh.ctx, util.ResourceEvent, key, uri, body)
}

func (eh *EventHandler) Delete(key, uri string, body []byte) *handler.Response {
	return handler.DoDelete(eh.ctx, util.ResourceEvent, key, uri, body)
}

func (eh *EventHandler) List(name, namespace, uri string) *handler.Response {
	// TODO list event
	return &handler.Response{
		Body: []byte(""),
	}
}

func (eh *EventHandler) Get(key string) *handler.Response {
	return handler.DoGet(eh.ctx, util.ResourceEvent, key)
}

func (eh *EventHandler) UpdatePut(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePut(eh.ctx, util.ResourceEvent, key, uri, body)
}

func (eh *EventHandler) UpdatePatch(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePatch(eh.ctx, util.ResourceEvent, key, uri, body)
}

func (eh *EventHandler) GetInitWatchEvent(fieldSelector, namespace, name string) []metav1.WatchEvent {
	return []metav1.WatchEvent{}
}
