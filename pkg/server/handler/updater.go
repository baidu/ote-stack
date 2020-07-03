package handler

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/emicklei/go-restful"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/util"
)

func (h *ServiceHandler) RegisterUpdateHandler(ws *restful.WebService) {
	ws.Route(ws.PATCH("namespaces/{namespace}/{resource}/{name}").To(h.updatePatchHandler))
	ws.Route(ws.PATCH("namespaces/{namespace}/{resource}/{name}/status").To(h.updatePatchHandler))
	ws.Route(ws.PATCH("{resource}/{name}/status").To(h.updatePatchHandler))
	ws.Route(ws.PUT("namespaces/{namespace}/{resource}/{name}").To(h.updatePutHandler))
	ws.Route(ws.PUT("{resource}/{name}").To(h.updatePutHandler))
}

func (h *ServiceHandler) updatePatchHandler(req *restful.Request, resp *restful.Response) {
	resource := req.PathParameter("resource")
	namespace := req.PathParameter("namespace")
	name := req.PathParameter("name")

	key := util.FormKeyName(namespace, name)

	httpBody, err := ioutil.ReadAll(req.Request.Body)
	if err != nil {
		klog.Errorf("http body read error:%v", err)
		resp.WriteError(http.StatusBadRequest, err)
		return
	}

	if _, ok := h.ServerHandler[resource]; !ok {
		klog.Errorf("unsupport resource: %s in patch operation.", resource)
		resp.WriteError(http.StatusBadRequest, fmt.Errorf("unsupport resource in patch operation."))
		return
	}

	result := h.ServerHandler[resource].UpdatePatch(key, req.Request.RequestURI, httpBody)
	if result.Err != nil {
		if !CheckResponseCode(result.Code) {
			result.Code = http.StatusInternalServerError
		}
		resp.WriteError(result.Code, result.Err)
		return
	}
	resp.Header().Set("Content-Type", "application/json")
	resp.Write(result.Body)

	klog.V(4).Infof("do patch %s-%s-%s success.", resource, namespace, name)
}

func (h *ServiceHandler) updatePutHandler(req *restful.Request, resp *restful.Response) {
	resource := req.PathParameter("resource")
	namespace := req.PathParameter("namespace")
	name := req.PathParameter("name")

	httpBody, err := ioutil.ReadAll(req.Request.Body)
	if err != nil {
		klog.Errorf("http body read error:%v", err)
		resp.WriteError(http.StatusBadRequest, err)
		return
	}

	key := util.FormKeyName(namespace, name)

	if _, ok := h.ServerHandler[resource]; !ok {
		klog.Errorf("unsupport resource: %s in put operation.", resource)
		resp.WriteError(http.StatusBadRequest, fmt.Errorf("unsupport resource in put operation."))
		return
	}

	result := h.ServerHandler[resource].UpdatePut(key, req.Request.RequestURI, httpBody)
	if result.Err != nil {
		if !CheckResponseCode(result.Code) {
			result.Code = http.StatusInternalServerError
		}
		resp.WriteError(result.Code, result.Err)
		return
	}
	resp.Header().Set("Content-Type", "application/json")
	resp.Write(result.Body)

	klog.V(4).Infof("do update %s-%s-%s success.", resource, namespace, name)
}

func DoUpdatePut(ctx *HandlerContext, objType, key, uri string, body []byte) *Response {
	if ctx.Lb.IsRemoteEnable() {
		var code int

		result := ctx.K8sClient.CoreV1().RESTClient().Put().RequestURI(uri).Body(body).Do()
		raw, err := result.Raw()
		result.StatusCode(&code)

		if err != nil {
			klog.Errorf("update %s failed: %v", objType, err)
			return &Response{
				Code: code,
				Err:  err,
			}
		}

		return &Response{
			Body: raw,
		}
	} else {
		err := ctx.Store.Update(objType, key, body)
		if err != nil {
			klog.Errorf("update %s failed: %v", objType, err)
			return &Response{
				Code: http.StatusInternalServerError,
				Err:  err,
			}
		}

		return &Response{
			Body: body,
		}
	}
}

func DoUpdatePatch(ctx *HandlerContext, objType, key, uri string, body []byte) *Response {
	if ctx.Lb.IsRemoteEnable() {
		var code int

		result := ctx.K8sClient.CoreV1().RESTClient().Patch(types.StrategicMergePatchType).RequestURI(uri).Body(body).Do()
		raw, err := result.Raw()
		result.StatusCode(&code)

		if err != nil {
			klog.Errorf("patch %s failed: %v", objType, err)
			return &Response{
				Code: code,
				Err:  err,
			}
		}

		return &Response{
			Body: raw,
		}
	} else {
		// TODO patch in local db
		data, err := ctx.Store.Get(objType, key, true)
		if err != nil {
			klog.Errorf("patch %s failed: %v", objType, err)
			return &Response{
				Code: http.StatusInternalServerError,
				Err:  err,
			}
		}

		return &Response{
			Body: data.([]byte),
		}
	}
}
