package resource

import (
	"net/http"

	"k8s.io/api/node/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/server/handler"
	"github.com/baidu/ote-stack/pkg/util"
)

type RuntimeClassHandler struct {
	ctx *handler.HandlerContext
}

func NewRuntimeClassHandler(ctx *handler.HandlerContext) handler.Handler {
	return &RuntimeClassHandler{
		ctx: ctx,
	}
}

func (rh *RuntimeClassHandler) Create(body []byte, uri string) *handler.Response {
	runtimeclass, err := util.GetObjectFromSerializeData(util.ResourceRuntimeClass, body)
	if err != nil {
		klog.Errorf("get object from serialized data failed: %v", err)
		return &handler.Response{
			Code: http.StatusBadRequest,
			Err:  err,
		}
	}

	key, err := cache.MetaNamespaceKeyFunc(runtimeclass)
	if err != nil {
		klog.Errorf("get runtimeclass key failed: %v", err)
		return &handler.Response{
			Code: http.StatusBadRequest,
			Err:  err,
		}
	}

	return handler.DoCreate(rh.ctx, util.ResourceRuntimeClass, key, uri, body)
}

func (rh *RuntimeClassHandler) Delete(key, uri string, body []byte) *handler.Response {
	return handler.DoDelete(rh.ctx, util.ResourceRuntimeClass, key, uri, body)
}

func (rh *RuntimeClassHandler) List(fieldSelector, namespace, uri string) *handler.Response {
	result := v1beta1.RuntimeClassList{}
	result.Kind = "RuntimeClassList"
	result.APIVersion = "node.k8s.io/v1beta1"
	result.SelfLink = uri

	list := rh.ctx.Store.List(util.ResourceRuntimeClass)

	for _, item := range list {
		runtimeclass := item.(*v1beta1.RuntimeClass)

		if namespace != "" && runtimeclass.Namespace != namespace {
			continue
		}

		if !handler.IsFilterFromQueryParams(fieldSelector, "", runtimeclass.Name) {
			klog.V(4).Infof("check FilterFromQueryParams failed")
			continue
		}

		result.Items = append(result.Items, *runtimeclass)
	}

	data, err := json.Marshal(result)
	if err != nil {
		klog.Errorf("json marshal failed: %v", err)
		return &handler.Response{
			Err:  err,
			Code: http.StatusInternalServerError,
		}
	}

	return &handler.Response{
		Body: data,
	}
}

func (rh *RuntimeClassHandler) Get(key string) *handler.Response {
	return handler.DoGet(rh.ctx, util.ResourceRuntimeClass, key)
}

func (rh *RuntimeClassHandler) UpdatePut(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePut(rh.ctx, util.ResourceRuntimeClass, key, uri, body)
}

func (rh *RuntimeClassHandler) UpdatePatch(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePatch(rh.ctx, util.ResourceRuntimeClass, key, uri, body)
}

func (rh *RuntimeClassHandler) GetInitWatchEvent(fieldSelector, namespace, name string) []metav1.WatchEvent {
	var result []metav1.WatchEvent
	list := rh.ctx.Store.List(util.ResourceRuntimeClass)

	for _, item := range list {
		runtimeclass := item.(*v1beta1.RuntimeClass)

		if namespace != "" && runtimeclass.Namespace != namespace {
			continue
		}

		if name != "" && runtimeclass.Name != name {
			continue
		}

		if !handler.IsFilterFromQueryParams(fieldSelector, "", runtimeclass.Name) {
			klog.V(4).Infof("check FilterFromQueryParams failed")
			continue
		}

		event := metav1.WatchEvent{
			Type:   string(watch.Added),
			Object: runtime.RawExtension{Object: runtimeclass},
		}
		result = append(result, event)
	}

	return result
}
