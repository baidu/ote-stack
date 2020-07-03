package resource

import (
	"net/http"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/server/handler"
	"github.com/baidu/ote-stack/pkg/util"
)

type ConfigMapHandler struct {
	ctx *handler.HandlerContext
}

func NewConfigMapHandler(ctx *handler.HandlerContext) handler.Handler {
	return &ConfigMapHandler{
		ctx: ctx,
	}
}

func (ch *ConfigMapHandler) Create(body []byte, uri string) *handler.Response {
	// TODO create configmap
	return handler.DoCreate(ch.ctx, util.ResourceConfigMap, "", uri, body)
}

func (ch *ConfigMapHandler) Delete(key, uri string, body []byte) *handler.Response {
	return handler.DoDelete(ch.ctx, util.ResourceConfigMap, key, uri, body)
}

func (ch *ConfigMapHandler) List(fieldSelector, namespace, uri string) *handler.Response {
	result := corev1.ConfigMapList{}
	result.Kind = "ConfigMapList"
	result.APIVersion = "v1"
	result.SelfLink = uri
	result.ResourceVersion = "1"

	list := ch.ctx.Store.List(util.ResourceConfigMap)

	for _, item := range list {
		configmap := item.(*corev1.ConfigMap)

		if namespace != "" && configmap.Namespace != namespace {
			continue
		}

		if !handler.IsFilterFromQueryParams(fieldSelector, "", configmap.Name) {
			klog.V(4).Infof("check FilterFromQueryParams failed")
			continue
		}

		result.Items = append(result.Items, *configmap)
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

func (ch *ConfigMapHandler) Get(key string) *handler.Response {
	return handler.DoGet(ch.ctx, util.ResourceConfigMap, key)
}

func (ch *ConfigMapHandler) UpdatePut(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePut(ch.ctx, util.ResourceConfigMap, key, uri, body)
}

func (ch *ConfigMapHandler) UpdatePatch(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePatch(ch.ctx, util.ResourceConfigMap, key, uri, body)
}

func (ch *ConfigMapHandler) GetInitWatchEvent(fieldSelector, namespace, name string) []metav1.WatchEvent {
	var result []metav1.WatchEvent
	list := ch.ctx.Store.List(util.ResourceConfigMap)

	for _, item := range list {
		configmap := item.(*corev1.ConfigMap)

		if namespace != "" && configmap.Namespace != namespace {
			continue
		}

		if name != "" && configmap.Name != name {
			continue
		}

		if !handler.IsFilterFromQueryParams(fieldSelector, "", configmap.Name) {
			klog.V(4).Infof("check FilterFromQueryParams failed")
			continue
		}

		event := metav1.WatchEvent{
			Type:   string(watch.Added),
			Object: runtime.RawExtension{Object: configmap},
		}
		result = append(result, event)
	}

	return result
}
