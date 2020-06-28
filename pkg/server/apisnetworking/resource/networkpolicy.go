package resource

import (
	"net/http"

	"k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/server/handler"
	"github.com/baidu/ote-stack/pkg/util"
)

type NetworkPolicyHandler struct {
	ctx *handler.HandlerContext
}

func NewNetworkPolicyHandler(ctx *handler.HandlerContext) handler.Handler {
	return &NetworkPolicyHandler{
		ctx: ctx,
	}
}

func (nh *NetworkPolicyHandler) Create(body []byte, uri string) *handler.Response {
	networkpolicy, err := util.GetObjectFromSerializeData(util.ResourceNetworkPolicy, body)
	if err != nil {
		klog.Errorf("get object from serialized data failed: %v", err)
		return &handler.Response{
			Code: http.StatusBadRequest,
			Err:  err,
		}
	}

	key, err := cache.MetaNamespaceKeyFunc(networkpolicy)
	if err != nil {
		klog.Errorf("get networkpolicy key failed: %v", err)
		return &handler.Response{
			Code: http.StatusBadRequest,
			Err:  err,
		}
	}

	return handler.DoCreate(nh.ctx, util.ResourceNetworkPolicy, key, uri, body)
}

func (nh *NetworkPolicyHandler) Delete(key, uri string, body []byte) *handler.Response {
	return handler.DoDelete(nh.ctx, util.ResourceNetworkPolicy, key, uri, body)
}

func (nh *NetworkPolicyHandler) List(fieldSelector, namespace, uri string) *handler.Response {
	result := v1.NetworkPolicyList{}
	result.Kind = "NetworkPolicyList"
	result.APIVersion = "networking.k8s.io/v1"
	result.SelfLink = uri

	list := nh.ctx.Store.List(util.ResourceNetworkPolicy)

	for _, item := range list {
		networkpolicy := item.(*v1.NetworkPolicy)

		if namespace != "" && networkpolicy.Namespace != namespace {
			continue
		}

		if !handler.IsFilterFromQueryParams(fieldSelector, "", networkpolicy.Name) {
			klog.V(4).Infof("check FilterFromQueryParams failed")
			continue
		}

		result.Items = append(result.Items, *networkpolicy)
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

func (nh *NetworkPolicyHandler) Get(key string) *handler.Response {
	return handler.DoGet(nh.ctx, util.ResourceNetworkPolicy, key)
}

func (nh *NetworkPolicyHandler) UpdatePut(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePut(nh.ctx, util.ResourceNetworkPolicy, key, uri, body)
}

func (nh *NetworkPolicyHandler) UpdatePatch(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePatch(nh.ctx, util.ResourceNetworkPolicy, key, uri, body)
}

func (nh *NetworkPolicyHandler) GetInitWatchEvent(fieldSelector, namespace, name string) []metav1.WatchEvent {
	var result []metav1.WatchEvent
	list := nh.ctx.Store.List(util.ResourceNetworkPolicy)

	for _, item := range list {
		networkpolicy := item.(*v1.NetworkPolicy)

		if namespace != "" && networkpolicy.Namespace != namespace {
			continue
		}

		if name != "" && networkpolicy.Name != name {
			continue
		}

		if !handler.IsFilterFromQueryParams(fieldSelector, "", networkpolicy.Name) {
			klog.V(4).Infof("check FilterFromQueryParams failed")
			continue
		}

		event := metav1.WatchEvent{
			Type:   string(watch.Added),
			Object: runtime.RawExtension{Object: networkpolicy},
		}
		result = append(result, event)
	}

	return result
}
