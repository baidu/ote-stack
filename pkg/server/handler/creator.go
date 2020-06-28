package handler

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/emicklei/go-restful"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog"
)

func (h *ServiceHandler) RegisterCreateHandler(ws *restful.WebService) {
	ws.Route(ws.POST("namespaces/{namespace}/{resource}").To(h.createHandler))
	ws.Route(ws.POST("{resource}").To(h.createHandler))
}

func (h *ServiceHandler) createHandler(req *restful.Request, resp *restful.Response) {
	resource := req.PathParameter("resource")

	httpBody, err := ioutil.ReadAll(req.Request.Body)
	if err != nil {
		klog.Errorf("http body read error:%v", err)
		resp.WriteError(http.StatusBadRequest, err)
		return
	}

	if _, ok := h.ServerHandler[resource]; !ok {
		klog.Errorf("unsupport resource: %s in create operation.", resource)
		resp.WriteError(http.StatusBadRequest, fmt.Errorf("unsupport resource in delete operation."))
		return
	}

	result := h.ServerHandler[resource].Create(httpBody, req.Request.RequestURI)
	if result.Err != nil {
		if !CheckResponseCode(result.Code) {
			result.Code = http.StatusInternalServerError
		}
		resp.WriteError(result.Code, result.Err)
		return
	}
	resp.Header().Set("Content-Type", "application/json")
	resp.Write(result.Body)

	klog.V(4).Infof("create %s success.", resource)
}

func DoCreate(ctx *HandlerContext, objType, key, uri string, body []byte) *Response {
	if ctx.Lb.IsRemoteEnable() {
		var code int

		result := ctx.K8sClient.CoreV1().RESTClient().Post().RequestURI(uri).Body(body).Do()
		raw, err := result.Raw()
		result.StatusCode(&code)

		if err != nil {
			klog.Errorf("create %s-%s failed: %v", objType, key, err)
			return &Response{
				Err:  err,
				Code: code,
			}
		}

		return &Response{
			Body: raw,
		}
	} else {
		if _, err := ctx.Store.Get(objType, key, false); err == nil {
			klog.Errorf("create %s-%s failed: it is already exist", objType, key)
			return &Response{
				Err:  apierrors.NewAlreadyExists(schema.GroupResource{}, key),
				Code: http.StatusConflict,
			}
		}
		err := ctx.Store.Update(objType, key, body)
		if err != nil {
			klog.Errorf("store %s failed: %v", objType, err)
			return &Response{
				Err:  err,
				Code: http.StatusInternalServerError,
			}
		}

		return &Response{
			Body: body,
		}
	}
}
