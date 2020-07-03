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

type SecretHandler struct {
	ctx *handler.HandlerContext
}

func NewSecretHandler(ctx *handler.HandlerContext) handler.Handler {
	return &SecretHandler{
		ctx: ctx,
	}
}

func (sh *SecretHandler) Create(body []byte, uri string) *handler.Response {
	// TODO create secret
	return handler.DoCreate(sh.ctx, util.ResourceSecret, "", uri, body)
}

func (sh *SecretHandler) Delete(key, uri string, body []byte) *handler.Response {
	return handler.DoDelete(sh.ctx, util.ResourceSecret, key, uri, body)
}

func (sh *SecretHandler) List(fieldSelector, namespace, uri string) *handler.Response {
	result := corev1.SecretList{}
	result.Kind = "SecretList"
	result.APIVersion = "v1"
	result.SelfLink = uri
	result.ResourceVersion = "1"

	list := sh.ctx.Store.List(util.ResourceSecret)

	for _, item := range list {
		secret := item.(*corev1.Secret)

		if namespace != "" && secret.Namespace != namespace {
			continue
		}

		if !handler.IsFilterFromQueryParams(fieldSelector, "", secret.Name) {
			klog.V(4).Infof("check FilterFromQueryParams failed")
			continue
		}

		result.Items = append(result.Items, *secret)
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

func (sh *SecretHandler) Get(key string) *handler.Response {
	return handler.DoGet(sh.ctx, util.ResourceSecret, key)
}

func (sh *SecretHandler) UpdatePut(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePut(sh.ctx, util.ResourceSecret, key, uri, body)
}

func (sh *SecretHandler) UpdatePatch(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePatch(sh.ctx, util.ResourceSecret, key, uri, body)
}

func (sh *SecretHandler) GetInitWatchEvent(fieldSelector, namespace, name string) []metav1.WatchEvent {
	var result []metav1.WatchEvent
	list := sh.ctx.Store.List(util.ResourceSecret)

	for _, item := range list {
		secret := item.(*corev1.Secret)

		if namespace != "" && secret.Namespace != namespace {
			continue
		}

		if name != "" && secret.Name != name {
			continue
		}

		if !handler.IsFilterFromQueryParams(fieldSelector, "", secret.Name) {
			klog.V(4).Infof("check FilterFromQueryParams failed")
			continue
		}

		event := metav1.WatchEvent{
			Type:   string(watch.Added),
			Object: runtime.RawExtension{Object: secret},
		}
		result = append(result, event)
	}

	return result
}
