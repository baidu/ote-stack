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

type PodHandler struct {
	ctx *handler.HandlerContext
}

func NewPodHandler(ctx *handler.HandlerContext) handler.Handler {
	return &PodHandler{
		ctx: ctx,
	}
}

func (ph *PodHandler) Create(body []byte, uri string) *handler.Response {
	pod, err := util.GetObjectFromSerializeData(util.ResourcePod, body)
	if err != nil {
		klog.Errorf("get object from serialized data failed: %v", err)
		return &handler.Response{
			Code: http.StatusBadRequest,
			Err:  err,
		}
	}

	key, err := cache.MetaNamespaceKeyFunc(pod)
	if err != nil {
		klog.Errorf("get pod key failed: %v", err)
		return &handler.Response{
			Code: http.StatusBadRequest,
			Err:  err,
		}
	}

	return handler.DoCreate(ph.ctx, util.ResourcePod, key, uri, body)
}

func (ph *PodHandler) Delete(key, uri string, body []byte) *handler.Response {
	return handler.DoDelete(ph.ctx, util.ResourcePod, key, uri, body)
}

func (ph *PodHandler) List(fieldSelector, namespace, uri string) *handler.Response {
	result := corev1.PodList{}
	result.Kind = "PodList"
	result.APIVersion = "v1"
	result.SelfLink = uri

	list := ph.ctx.Store.List(util.ResourcePod)

	for _, item := range list {
		pod := item.(*corev1.Pod)

		if namespace != "" && pod.Namespace != namespace {
			continue
		}

		if !handler.IsFilterFromQueryParams(fieldSelector, pod.Spec.NodeName, pod.Name) {
			klog.V(4).Infof("check FilterFromQueryParams failed")
			continue
		}

		result.Items = append(result.Items, *pod)
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

func (ph *PodHandler) Get(key string) *handler.Response {
	return handler.DoGet(ph.ctx, util.ResourcePod, key)
}

func (ph *PodHandler) UpdatePut(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePut(ph.ctx, util.ResourcePod, key, uri, body)
}

func (ph *PodHandler) UpdatePatch(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePatch(ph.ctx, util.ResourcePod, key, uri, body)
}

func (ph *PodHandler) GetInitWatchEvent(fieldSelector, namespace, name string) []metav1.WatchEvent {
	var result []metav1.WatchEvent
	list := ph.ctx.Store.List(util.ResourcePod)

	for _, item := range list {
		pod := item.(*corev1.Pod)

		if namespace != "" && pod.Namespace != namespace {
			continue
		}

		if name != "" && pod.Name != name {
			continue
		}

		if !handler.IsFilterFromQueryParams(fieldSelector, pod.Spec.NodeName, pod.Name) {
			klog.V(4).Infof("check FilterFromQueryParams failed")
			continue
		}

		event := metav1.WatchEvent{
			Type:   string(watch.Added),
			Object: runtime.RawExtension{Object: pod},
		}
		result = append(result, event)
	}

	return result
}
