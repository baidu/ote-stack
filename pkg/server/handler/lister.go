package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/emicklei/go-restful"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/util"
)

func (h *ServiceHandler) RegisterListHandler(ws *restful.WebService) {
	ws.Route(ws.GET("namespaces/{namespace}/{resource}").To(h.listHandler))
	ws.Route(ws.GET("{resource}").To(h.listHandler))
	ws.Route(ws.GET("namespaces/{namespace}/{resource}/{name}").To(h.getHandler))
	ws.Route(ws.GET("{resource}/{name}").To(h.getHandler))
}

func (h *ServiceHandler) listHandler(req *restful.Request, resp *restful.Response) {
	resource := req.PathParameter("resource")
	namespace := req.PathParameter("namespace")
	isWatch := req.QueryParameter("watch")
	fieldSelector := req.QueryParameter("fieldSelector")
	// TODO filter by labels

	if isWatch == "true" {
		h.watchHandler(req, resp)
		return
	}

	if _, ok := h.ServerHandler[resource]; !ok {
		klog.Errorf("unsupport resource: %s in list operation.", resource)
		resp.WriteError(http.StatusBadRequest, fmt.Errorf("unsupport resource in list operation."))
		return
	}

	result := h.ServerHandler[resource].List(fieldSelector, namespace, strings.Split(req.Request.RequestURI, "?")[0])
	if result.Err != nil {
		if !CheckResponseCode(result.Code) {
			result.Code = http.StatusInternalServerError
		}
		resp.WriteError(result.Code, result.Err)
		return
	}
	resp.Header().Set("Content-Type", "application/json")
	resp.Write(result.Body)

	klog.V(4).Infof("list %s-%s success.", resource, namespace)
}

func (h *ServiceHandler) getHandler(req *restful.Request, resp *restful.Response) {
	resource := req.PathParameter("resource")
	namespace := req.PathParameter("namespace")
	name := req.PathParameter("name")
	isWatch := req.QueryParameter("watch")

	if isWatch == "true" {
		h.watchHandler(req, resp)
		return
	}

	key := util.FormKeyName(namespace, name)

	if _, ok := h.ServerHandler[resource]; !ok {
		klog.Errorf("unsupport resource: %s in get operation.", resource)
		resp.WriteError(http.StatusBadRequest, fmt.Errorf("unsupport resource in get operation."))
		return
	}

	result := h.ServerHandler[resource].Get(key)
	if result.Err != nil {
		if !CheckResponseCode(result.Code) {
			result.Code = http.StatusInternalServerError
		}
		resp.WriteError(result.Code, result.Err)
		return
	}

	// when k3s agent get kubernetes endpoint, it should change the endpoint to point to edgehub
	if req.Request.RequestURI == "/api/v1/namespaces/default/endpoints/kubernetes" {
		result.Body = handleK3sGetEndpoint(result.Body)
	}

	resp.Header().Set("Content-Type", "application/json")
	resp.Write(result.Body)

	klog.V(4).Infof("get %s-%s-%s success.", resource, namespace, name)
}

func DoGet(ctx *HandlerContext, objType, key string) *Response {
	data, err := ctx.Store.Get(objType, key, true)
	if err != nil {
		klog.Errorf("get %s storage failed: %v", objType, err)
		return &Response{
			Code: http.StatusNotFound,
			Err:  apierrors.NewNotFound(schema.GroupResource{}, key),
		}
	} else {
		return &Response{
			Body: data.([]byte),
		}
	}
}

func handleK3sGetEndpoint(data []byte) []byte {
	endpoint := &corev1.Endpoints{}
	if err := json.Unmarshal(data, endpoint); err != nil {
		klog.Errorf("json unmarshal failed: %v", err)
		return nil
	}

	changeEdgehubEndpoint(endpoint)

	ret, err := json.Marshal(endpoint)
	if err != nil {
		klog.Errorf("json marshal failed: %v", err)
		return nil
	}
	return ret
}

// changeEdgehubEndpoint makes endpoint points to edgehub address.
func changeEdgehubEndpoint(ep *corev1.Endpoints) {
	sub := corev1.EndpointSubset{
		Addresses: []corev1.EndpointAddress{{IP: "127.0.0.1"}},
		Ports: []corev1.EndpointPort{{
			Name:     "https",
			Port:     8778,
			Protocol: "TCP",
		}},
	}

	ep.Subsets = []corev1.EndpointSubset{sub}
}
