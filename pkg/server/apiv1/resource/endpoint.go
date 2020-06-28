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

type EndpointHandler struct {
	ctx *handler.HandlerContext
}

func NewEndpointHandler(ctx *handler.HandlerContext) handler.Handler {
	return &EndpointHandler{
		ctx: ctx,
	}
}

func (eh *EndpointHandler) Create(body []byte, uri string) *handler.Response {
	endpoint, err := util.GetObjectFromSerializeData(util.ResourceEndpoint, body)
	if err != nil {
		klog.Errorf("get object from serialized data failed: %v", err)
		return &handler.Response{
			Code: http.StatusBadRequest,
			Err:  err,
		}
	}

	key, err := cache.MetaNamespaceKeyFunc(endpoint)
	if err != nil {
		klog.Errorf("get endpoint key failed: %v", err)
		return &handler.Response{
			Code: http.StatusBadRequest,
			Err:  err,
		}
	}

	return handler.DoCreate(eh.ctx, util.ResourceEndpoint, key, uri, body)
}

func (eh *EndpointHandler) Delete(key, uri string, body []byte) *handler.Response {
	return handler.DoDelete(eh.ctx, util.ResourceEndpoint, key, uri, body)
}

func (eh *EndpointHandler) List(fieldSelector, namespace, uri string) *handler.Response {
	result := corev1.EndpointsList{}
	result.Kind = "EndpointsList"
	result.APIVersion = "v1"
	result.SelfLink = uri

	list := eh.ctx.Store.List(util.ResourceEndpoint)

	for _, item := range list {
		endpoint := item.(*corev1.Endpoints)

		if namespace != "" && endpoint.Namespace != namespace {
			continue
		}

		if !handler.IsFilterFromQueryParams(fieldSelector, "", endpoint.Name) {
			klog.V(4).Infof("check FilterFromQueryParams failed")
			continue
		}

		result.Items = append(result.Items, *endpoint)
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

func (eh *EndpointHandler) Get(key string) *handler.Response {
	return handler.DoGet(eh.ctx, util.ResourceEndpoint, key)
}

func (eh *EndpointHandler) UpdatePut(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePut(eh.ctx, util.ResourceEndpoint, key, uri, body)
}

func (eh *EndpointHandler) UpdatePatch(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePatch(eh.ctx, util.ResourceEndpoint, key, uri, body)
}

func (eh *EndpointHandler) GetInitWatchEvent(fieldSelector, namespace, name string) []metav1.WatchEvent {
	var result []metav1.WatchEvent

	// when k3s agent watches kubernetes endpoint, edgehub is no need to send the changed endpoint.
	if namespace == "default" && fieldSelector == "metadata.name=kubernetes" {
		return result
	}

	list := eh.ctx.Store.List(util.ResourceEndpoint)

	for _, item := range list {
		endpoint := item.(*corev1.Endpoints)

		if namespace != "" && endpoint.Namespace != namespace {
			continue
		}

		if name != "" && endpoint.Name != name {
			continue
		}

		if !handler.IsFilterFromQueryParams(fieldSelector, "", endpoint.Name) {
			klog.V(4).Infof("check FilterFromQueryParams failed")
			continue
		}

		event := metav1.WatchEvent{
			Type:   string(watch.Added),
			Object: runtime.RawExtension{Object: endpoint},
		}
		result = append(result, event)
	}

	return result
}
