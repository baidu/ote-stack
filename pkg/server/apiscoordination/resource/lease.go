package resource

import (
	"net/http"

	"k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/server/handler"
	"github.com/baidu/ote-stack/pkg/util"
)

type LeaseHandler struct {
	ctx *handler.HandlerContext
}

func NewLeaseHandler(ctx *handler.HandlerContext) handler.Handler {
	return &LeaseHandler{
		ctx: ctx,
	}
}

func (lh *LeaseHandler) Create(body []byte, uri string) *handler.Response {
	lease, err := util.GetObjectFromSerializeData(util.ResourceNodeLease, body)
	if err != nil {
		klog.Errorf("get object from serialized data failed: %v", err)
		return &handler.Response{
			Code: http.StatusBadRequest,
			Err:  err,
		}
	}

	key, err := cache.MetaNamespaceKeyFunc(lease)
	if err != nil {
		klog.Errorf("get lease key failed: %v", err)
		return &handler.Response{
			Code: http.StatusBadRequest,
			Err:  err,
		}
	}

	return handler.DoCreate(lh.ctx, util.ResourceNodeLease, key, uri, body)
}

func (lh *LeaseHandler) Delete(key, uri string, body []byte) *handler.Response {
	return handler.DoDelete(lh.ctx, util.ResourceNodeLease, key, uri, body)
}

func (lh *LeaseHandler) List(fieldSelector, namespace, uri string) *handler.Response {
	result := v1.LeaseList{}
	result.Kind = "LeaseList"
	result.APIVersion = "coordination.k8s.io/v1"
	result.SelfLink = uri

	list := lh.ctx.Store.List(util.ResourceNodeLease)

	for _, item := range list {
		lease := item.(*v1.Lease)

		if namespace != "" && lease.Namespace != namespace {
			continue
		}

		if !handler.IsFilterFromQueryParams(fieldSelector, "", lease.Name) {
			klog.V(4).Infof("check FilterFromQueryParams failed")
			continue
		}

		result.Items = append(result.Items, *lease)
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

func (lh *LeaseHandler) Get(key string) *handler.Response {
	return handler.DoGet(lh.ctx, util.ResourceNodeLease, key)
}

func (lh *LeaseHandler) UpdatePut(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePut(lh.ctx, util.ResourceNodeLease, key, uri, body)
}

func (lh *LeaseHandler) UpdatePatch(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePatch(lh.ctx, util.ResourceNodeLease, key, uri, body)
}

func (lh *LeaseHandler) GetInitWatchEvent(fieldSelector, namespace, name string) []metav1.WatchEvent {
	var result []metav1.WatchEvent
	list := lh.ctx.Store.List(util.ResourceNodeLease)

	for _, item := range list {
		lease := item.(*v1.Lease)

		if namespace != "" && lease.Namespace != namespace {
			continue
		}

		if name != "" && lease.Name != name {
			continue
		}

		if !handler.IsFilterFromQueryParams(fieldSelector, "", lease.Name) {
			klog.V(4).Infof("check FilterFromQueryParams failed")
			continue
		}

		event := metav1.WatchEvent{
			Type:   string(watch.Added),
			Object: runtime.RawExtension{Object: lease},
		}
		result = append(result, event)
	}

	return result
}
