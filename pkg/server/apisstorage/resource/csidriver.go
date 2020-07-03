package resource

import (
	"net/http"

	"k8s.io/api/storage/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/server/handler"
	"github.com/baidu/ote-stack/pkg/util"
)

type CSIDriverHandler struct {
	ctx *handler.HandlerContext
}

func NewCSIDriverHandler(ctx *handler.HandlerContext) handler.Handler {
	return &CSIDriverHandler{
		ctx: ctx,
	}
}

func (ch *CSIDriverHandler) Create(body []byte, uri string) *handler.Response {
	csidriver, err := util.GetObjectFromSerializeData(util.ResourceCSIDriver, body)
	if err != nil {
		klog.Errorf("get object from serialized data failed: %v", err)
		return &handler.Response{
			Code: http.StatusBadRequest,
			Err:  err,
		}
	}

	key, err := cache.MetaNamespaceKeyFunc(csidriver)
	if err != nil {
		klog.Errorf("get csidriver key failed: %v", err)
		return &handler.Response{
			Code: http.StatusBadRequest,
			Err:  err,
		}
	}

	return handler.DoCreate(ch.ctx, util.ResourceCSIDriver, key, uri, body)
}

func (ch *CSIDriverHandler) Delete(key, uri string, body []byte) *handler.Response {
	return handler.DoDelete(ch.ctx, util.ResourceCSIDriver, key, uri, body)
}

func (ch *CSIDriverHandler) List(fieldSelector, namespace, uri string) *handler.Response {
	result := v1beta1.CSIDriverList{}
	result.Kind = "CSIDriverList"
	result.APIVersion = "storage.k8s.io/v1beta1"
	result.SelfLink = uri

	list := ch.ctx.Store.List(util.ResourceCSIDriver)

	for _, item := range list {
		csidriver := item.(*v1beta1.CSIDriver)

		if namespace != "" && csidriver.Namespace != namespace {
			continue
		}

		if !handler.IsFilterFromQueryParams(fieldSelector, "", csidriver.Name) {
			klog.V(4).Infof("check FilterFromQueryParams failed")
			continue
		}

		result.Items = append(result.Items, *csidriver)
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

func (ch *CSIDriverHandler) Get(key string) *handler.Response {
	return handler.DoGet(ch.ctx, util.ResourceCSIDriver, key)
}

func (ch *CSIDriverHandler) UpdatePut(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePut(ch.ctx, util.ResourceCSIDriver, key, uri, body)
}

func (ch *CSIDriverHandler) UpdatePatch(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePatch(ch.ctx, util.ResourceCSIDriver, key, uri, body)
}

func (ch *CSIDriverHandler) GetInitWatchEvent(fieldSelector, namespace, name string) []metav1.WatchEvent {
	var result []metav1.WatchEvent
	list := ch.ctx.Store.List(util.ResourceCSIDriver)

	for _, item := range list {
		csidriver := item.(*v1beta1.CSIDriver)

		if namespace != "" && csidriver.Namespace != namespace {
			continue
		}

		if name != "" && csidriver.Name != name {
			continue
		}

		if !handler.IsFilterFromQueryParams(fieldSelector, "", csidriver.Name) {
			klog.V(4).Infof("check FilterFromQueryParams failed")
			continue
		}

		event := metav1.WatchEvent{
			Type:   string(watch.Added),
			Object: runtime.RawExtension{Object: csidriver},
		}
		result = append(result, event)
	}

	return result
}
