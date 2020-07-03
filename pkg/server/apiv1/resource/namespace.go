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

type NamespaceHandler struct {
	ctx *handler.HandlerContext
}

func NewNamespaceHandler(ctx *handler.HandlerContext) handler.Handler {
	return &NamespaceHandler{
		ctx: ctx,
	}
}

func (nh *NamespaceHandler) Create(body []byte, uri string) *handler.Response {
	namespace, err := util.GetObjectFromSerializeData(util.ResourceNamespace, body)
	if err != nil {
		klog.Errorf("get object from serialized data failed: %v", err)
		return &handler.Response{
			Code: http.StatusBadRequest,
			Err:  err,
		}
	}

	key, err := cache.MetaNamespaceKeyFunc(namespace)
	if err != nil {
		klog.Errorf("get namespace key failed: %v", err)
		return &handler.Response{
			Code: http.StatusBadRequest,
			Err:  err,
		}
	}

	return handler.DoCreate(nh.ctx, util.ResourceNamespace, key, uri, body)
}

func (nh *NamespaceHandler) Delete(key, uri string, body []byte) *handler.Response {
	return handler.DoDelete(nh.ctx, util.ResourceNamespace, key, uri, body)
}

func (nh *NamespaceHandler) List(fieldSelector, namespaces, uri string) *handler.Response {
	result := corev1.NamespaceList{}
	result.Kind = "NamespaceList"
	result.APIVersion = "v1"
	result.SelfLink = uri

	list := nh.ctx.Store.List(util.ResourceNamespace)

	for _, item := range list {
		namespace := item.(*corev1.Namespace)

		if !handler.IsFilterFromQueryParams(fieldSelector, "", namespace.Name) {
			klog.V(4).Infof("check FilterFromQueryParams failed")
			continue
		}

		result.Items = append(result.Items, *namespace)
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

func (nh *NamespaceHandler) Get(key string) *handler.Response {
	return handler.DoGet(nh.ctx, util.ResourceNamespace, key)
}

func (nh *NamespaceHandler) UpdatePut(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePut(nh.ctx, util.ResourceNamespace, key, uri, body)
}

func (nh *NamespaceHandler) UpdatePatch(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePatch(nh.ctx, util.ResourceNamespace, key, uri, body)
}

func (nh *NamespaceHandler) GetInitWatchEvent(fieldSelector, namespace, name string) []metav1.WatchEvent {
	var result []metav1.WatchEvent
	list := nh.ctx.Store.List(util.ResourceNamespace)

	for _, item := range list {
		namespace := item.(*corev1.Namespace)

		if name != "" && namespace.Name != name {
			continue
		}

		if !handler.IsFilterFromQueryParams(fieldSelector, "", namespace.Name) {
			klog.V(4).Infof("check FilterFromQueryParams failed")
			continue
		}

		event := metav1.WatchEvent{
			Type:   string(watch.Added),
			Object: runtime.RawExtension{Object: namespace},
		}
		result = append(result, event)
	}

	return result
}
