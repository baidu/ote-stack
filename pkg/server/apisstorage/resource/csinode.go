package resource

import (
	"net/http"

	"k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/server/handler"
	"github.com/baidu/ote-stack/pkg/util"
)

type CSINodeHandler struct {
	ctx *handler.HandlerContext
}

func NewCSINodeHandler(ctx *handler.HandlerContext) handler.Handler {
	return &CSINodeHandler{
		ctx: ctx,
	}
}

func (ch *CSINodeHandler) Create(body []byte, uri string) *handler.Response {
	csinode, err := util.GetObjectFromSerializeData(util.ResourceCSINode, body)
	if err != nil {
		klog.Errorf("get object from serialized data failed: %v", err)
		return &handler.Response{
			Code: http.StatusBadRequest,
			Err:  err,
		}
	}

	key, err := cache.MetaNamespaceKeyFunc(csinode)
	if err != nil {
		klog.Errorf("get csinode key failed: %v", err)
		return &handler.Response{
			Code: http.StatusBadRequest,
			Err:  err,
		}
	}

	return handler.DoCreate(ch.ctx, util.ResourceCSINode, key, uri, body)
}

func (ch *CSINodeHandler) Delete(key, uri string, body []byte) *handler.Response {
	return handler.DoDelete(ch.ctx, util.ResourceCSINode, key, uri, body)
}

func (ch *CSINodeHandler) List(fieldSelector, namespace, uri string) *handler.Response {
	result := v1.CSINodeList{}
	result.Kind = "CSINodeList"
	result.APIVersion = "storage.k8s.io/v1"
	result.SelfLink = uri

	list := ch.ctx.Store.List(util.ResourceCSINode)

	for _, item := range list {
		csinode := item.(*v1.CSINode)

		if namespace != "" && csinode.Namespace != namespace {
			continue
		}

		if !handler.IsFilterFromQueryParams(fieldSelector, "", csinode.Name) {
			klog.V(4).Infof("check FilterFromQueryParams failed")
			continue
		}

		result.Items = append(result.Items, *csinode)
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

func (ch *CSINodeHandler) Get(key string) *handler.Response {
	return handler.DoGet(ch.ctx, util.ResourceCSINode, key)
}

func (ch *CSINodeHandler) UpdatePut(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePut(ch.ctx, util.ResourceCSINode, key, uri, body)
}

func (ch *CSINodeHandler) UpdatePatch(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePatch(ch.ctx, util.ResourceCSINode, key, uri, body)
}

func (ch *CSINodeHandler) GetInitWatchEvent(fieldSelector, namespace, name string) []metav1.WatchEvent {
	var result []metav1.WatchEvent
	list := ch.ctx.Store.List(util.ResourceCSINode)

	for _, item := range list {
		csinode := item.(*v1.CSINode)

		if namespace != "" && csinode.Namespace != namespace {
			continue
		}

		if name != "" && csinode.Name != name {
			continue
		}

		if !handler.IsFilterFromQueryParams(fieldSelector, "", csinode.Name) {
			klog.V(4).Infof("check FilterFromQueryParams failed")
			continue
		}

		event := metav1.WatchEvent{
			Type:   string(watch.Added),
			Object: runtime.RawExtension{Object: csinode},
		}
		result = append(result, event)
	}

	return result
}
