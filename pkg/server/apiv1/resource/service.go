package resource

import (
	"net/http"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/server/handler"
	"github.com/baidu/ote-stack/pkg/util"
)

type ServiceHandler struct {
	ctx *handler.HandlerContext
}

func NewServiceHandler(ctx *handler.HandlerContext) handler.Handler {
	return &ServiceHandler{
		ctx: ctx,
	}
}

func (sh *ServiceHandler) Create(body []byte, uri string) *handler.Response {
	service, err := util.GetObjectFromSerializeData(util.ResourceService, body)
	if err != nil {
		klog.Errorf("get object from serialized data failed: %v", err)
		return &handler.Response{
			Code: http.StatusBadRequest,
			Err:  err,
		}
	}

	key, err := cache.MetaNamespaceKeyFunc(service)
	if err != nil {
		klog.Errorf("get service key failed: %v", err)
		return &handler.Response{
			Code: http.StatusBadRequest,
			Err:  err,
		}
	}

	return handler.DoCreate(sh.ctx, util.ResourceService, key, uri, body)
}

func (sh *ServiceHandler) Delete(key, uri string, body []byte) *handler.Response {
	return handler.DoDelete(sh.ctx, util.ResourceService, key, uri, body)
}

func (sh *ServiceHandler) List(fieldSelector, namespace, uri string) *handler.Response {
	result := corev1.ServiceList{}
	result.Kind = "ServiceList"
	result.APIVersion = "v1"
	result.SelfLink = uri

	list := sh.ctx.Store.List(util.ResourceService)

	for _, item := range list {
		service := item.(*corev1.Service)
		if namespace != "" && service.Namespace != namespace {
			continue
		}

		if !handler.IsFilterFromQueryParams(fieldSelector, "", service.Name) {
			klog.V(4).Infof("check FilterFromQueryParams failed")
			continue
		}

		result.Items = append(result.Items, *service)
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

func (sh *ServiceHandler) Get(key string) *handler.Response {
	return handler.DoGet(sh.ctx, util.ResourceService, key)
}

func (sh *ServiceHandler) UpdatePut(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePut(sh.ctx, util.ResourceService, key, uri, body)
}

func (sh *ServiceHandler) UpdatePatch(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePatch(sh.ctx, util.ResourceService, key, uri, body)
}

func (sh *ServiceHandler) GetInitWatchEvent(fieldSelector, namespace, name string) []metav1.WatchEvent {
	var result []metav1.WatchEvent
	list := sh.ctx.Store.List(util.ResourceService)

	for _, item := range list {
		service := item.(*corev1.Service)

		if namespace != "" && service.Namespace != namespace {
			continue
		}

		if name != "" && service.Name != name {
			continue
		}

		if !handler.IsFilterFromQueryParams(fieldSelector, "", service.Name) {
			klog.V(4).Infof("check FilterFromQueryParams failed")
			continue
		}

		event := metav1.WatchEvent{
			Type:   string(watch.Added),
			Object: runtime.RawExtension{Object: service},
		}
		result = append(result, event)
	}

	return result
}
