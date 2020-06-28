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

type NodeHandler struct {
	ctx *handler.HandlerContext
}

func NewNodeHandler(ctx *handler.HandlerContext) handler.Handler {
	return &NodeHandler{
		ctx: ctx,
	}
}

func (nh *NodeHandler) Create(body []byte, uri string) *handler.Response {
	node, err := util.GetObjectFromSerializeData(util.ResourceNode, body)
	if err != nil {
		klog.Errorf("get object from serialized data failed: %v", err)
		return &handler.Response{
			Code: http.StatusBadRequest,
			Err:  err,
		}
	}

	key, err := cache.MetaNamespaceKeyFunc(node)
	if err != nil {
		klog.Errorf("get node key failed: %v", err)
		return &handler.Response{
			Code: http.StatusBadRequest,
			Err:  err,
		}
	}

	return handler.DoCreate(nh.ctx, util.ResourceNode, key, uri, body)
}

func (nh *NodeHandler) Delete(key, uri string, body []byte) *handler.Response {
	return handler.DoDelete(nh.ctx, util.ResourceNode, key, uri, body)
}

func (nh *NodeHandler) List(fieldSelector, namespace, uri string) *handler.Response {
	result := corev1.NodeList{}
	result.Kind = "NodeList"
	result.APIVersion = "v1"
	result.SelfLink = uri

	list := nh.ctx.Store.List(util.ResourceNode)

	for _, item := range list {
		node := item.(*corev1.Node)

		if !handler.IsFilterFromQueryParams(fieldSelector, "", node.Name) {
			klog.V(4).Infof("check FilterFromQueryParams failed")
			continue
		}

		result.Items = append(result.Items, *node)
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

func (nh *NodeHandler) Get(key string) *handler.Response {
	return handler.DoGet(nh.ctx, util.ResourceNode, key)
}

func (nh *NodeHandler) UpdatePut(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePut(nh.ctx, util.ResourceNode, key, uri, body)
}

func (nh *NodeHandler) UpdatePatch(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePatch(nh.ctx, util.ResourceNode, key, uri, body)
}

func (nh *NodeHandler) GetInitWatchEvent(fieldSelector, namespace, name string) []metav1.WatchEvent {
	var result []metav1.WatchEvent
	list := nh.ctx.Store.List(util.ResourceNode)

	for _, item := range list {
		node := item.(*corev1.Node)

		if name != "" && node.Name != name {
			continue
		}

		if !handler.IsFilterFromQueryParams(fieldSelector, "", node.Name) {
			klog.V(4).Infof("check FilterFromQueryParams failed")
			continue
		}

		event := metav1.WatchEvent{
			Type:   string(watch.Added),
			Object: runtime.RawExtension{Object: node},
		}
		result = append(result, event)
	}

	return result
}
