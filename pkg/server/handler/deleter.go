package handler

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/emicklei/go-restful"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/util"
)

func (h *ServiceHandler) RegisterDeleteHandler(ws *restful.WebService) {
	ws.Route(ws.DELETE("namespaces/{namespace}/{resource}/{name}").To(h.deleteHandler))
}

func (h *ServiceHandler) deleteHandler(req *restful.Request, resp *restful.Response) {
	namespace := req.PathParameter("namespace")
	resource := req.PathParameter("resource")
	name := req.PathParameter("name")

	key := util.FormKeyName(namespace, name)

	httpBody, err := ioutil.ReadAll(req.Request.Body)
	if err != nil {
		klog.Errorf("http body read error:%v", err)
		resp.WriteError(http.StatusBadRequest, err)
		return
	}

	if _, ok := h.ServerHandler[resource]; !ok {
		klog.Errorf("unsupport resource: %s in delete operation.", resource)
		resp.WriteError(http.StatusBadRequest, fmt.Errorf("unsupport resource in delete operation."))
		return
	}

	result := h.ServerHandler[resource].Delete(key, req.Request.RequestURI, httpBody)
	if result.Err != nil {
		if !CheckResponseCode(result.Code) {
			result.Code = http.StatusInternalServerError
		}
		resp.WriteError(result.Code, result.Err)
		return
	}
	resp.Header().Set("Content-Type", "application/json")
	resp.Write(result.Body)

	klog.V(4).Infof("delete %s-%s-%s success.", resource, namespace, name)
}

func DoDelete(ctx *HandlerContext, objType, key, uri string, body []byte) *Response {
	if ctx.Lb.IsRemoteEnable() {
		var code int

		result := ctx.K8sClient.CoreV1().RESTClient().Delete().RequestURI(uri).Body(body).Do()
		raw, err := result.Raw()
		result.StatusCode(&code)

		if err != nil {
			klog.Errorf("delete %s failed: %v", objType, err)
			return &Response{
				Code: code,
				Err:  err,
			}
		}

		return &Response{
			Body: raw,
		}
	} else {
		err := ctx.Store.Delete(objType, key)
		if err != nil {
			klog.Errorf("delete %s failed: %v", objType, err)
			return &Response{
				Code: http.StatusInternalServerError,
				Err:  err,
			}
		}

		return &Response{
			Code: http.StatusOK,
		}
	}
}
