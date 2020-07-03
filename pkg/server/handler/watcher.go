package handler

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/emicklei/go-restful"
	"golang.org/x/net/websocket"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/util"
)

func (h *ServiceHandler) watchHandler(req *restful.Request, resp *restful.Response) {
	resource := req.PathParameter("resource")
	namespace := req.PathParameter("namespace")
	name := req.PathParameter("name")
	fieldSelector := req.QueryParameter("fieldSelector")
	// TODO filter by labels

	// handle websocket
	if isWebSocketRequest(req.Request) {
		klog.Infof("it is websocket")
		websocket.Handler(func(ws *websocket.Conn) {
			h.handleWS(ws, resource, fieldSelector, namespace, name)
		}).ServeHTTP(resp.ResponseWriter, req.Request)
		return
	}

	if _, ok := h.ServerHandler[resource]; !ok {
		klog.Errorf("unsupport resource: %s in watch operation.", resource)
		resp.WriteError(http.StatusBadRequest, fmt.Errorf("unsupport resource in watch operation."))
		return
	}

	h.doChunkWatch(h.Ctx, resource, fieldSelector, namespace, name, resp)
}

func isWebSocketRequest(req *http.Request) bool {
	if !strings.EqualFold(req.Header.Get("Upgrade"), "websocket") {
		return false
	}
	return connectionUpgradeRegex.MatchString(strings.ToLower(req.Header.Get("Connection")))
}

func (h *ServiceHandler) handleWS(ws *websocket.Conn, resource, fieldSelector, namespace, name string) {
	if _, ok := h.ServerHandler[resource]; !ok {
		klog.Errorf("unsupport resource: %s in watch operation.", resource)
		ws.Close()
		return
	}
	h.doWebsocketWatch(h.Ctx, resource, fieldSelector, namespace, name, ws)
}

func (h *ServiceHandler) doChunkWatch(ctx *HandlerContext, objType, fieldSelector, namespace, name string, resp *restful.Response) {
	w := resp.ResponseWriter
	flusher, ok := w.(http.Flusher)
	if !ok {
		klog.Errorf("failed to get flush")
		resp.WriteError(http.StatusInternalServerError, fmt.Errorf("failed to get flush"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// send the init watch event
	watchEventList := h.ServerHandler[objType].GetInitWatchEvent(fieldSelector, namespace, name)
	for _, event := range watchEventList {
		data, err := json.Marshal(event)
		if err != nil {
			klog.Errorf("json marshal watch event failed: %v", err)
			continue
		}

		fmt.Fprintf(w, "%s\n", string(data))
	}
	flusher.Flush()

	// set watch channel
	newChan := make(chan metav1.WatchEvent)
	key := util.GetUniqueId()
	ctx.EdgeSubscriber.Add(objType, key, newChan)

	isConnected := true
	for isConnected {
		select {
		case event := <-newChan:
			// check if this event is needed
			if !checkIsNeedToWatch(objType, fieldSelector, namespace, name, event.Object.Object) {
				klog.V(5).Infof("fieldSelector %s don't match %s-%s-%s", fieldSelector, objType, namespace, name)
				break
			}

			data, err := json.Marshal(event)
			if err != nil {
				klog.Errorf("get watch event failed: %v", err)
				continue
			}

			fmt.Fprintf(w, "%s\n", string(data))

			if len(newChan) == 0 {
				flusher.Flush()
			}
		case <-w.(http.CloseNotifier).CloseNotify():
			klog.Infof("watch %s-%s-%s connection closed", objType, namespace, name)
			isConnected = false
		}
	}
	ctx.EdgeSubscriber.Delete(objType, key)
}

func (h *ServiceHandler) doWebsocketWatch(ctx *HandlerContext, objType, fieldSelector, namespace, name string, ws *websocket.Conn) {
	// set watch channel
	newChan := make(chan metav1.WatchEvent)
	key := strconv.FormatInt(time.Now().Unix(), 10)
	ctx.EdgeSubscriber.Add(objType, key, newChan)

	// send the init watch event
	watchEventList := h.ServerHandler[objType].GetInitWatchEvent(fieldSelector, namespace, name)
	for _, event := range watchEventList {
		data, err := json.Marshal(event)
		if err != nil {
			klog.Errorf("json marshal watch event failed: %v", err)
			continue
		}

		if err := websocket.Message.Send(ws, data); err != nil {
			klog.Errorf("send message error:%v", err)
		}
	}

	isConnected := true
	for isConnected {
		select {
		case event := <-newChan:
			// check if this event is needed
			if !checkIsNeedToWatch(objType, fieldSelector, namespace, name, event.Object.Object) {
				klog.V(5).Infof("fieldSelector %s don't match %s-%s-%s", fieldSelector, objType, namespace, name)
				break
			}

			data, err := json.Marshal(event)
			if err != nil {
				klog.Errorf("get watch event failed: %v", err)
				continue
			}

			if err := websocket.Message.Send(ws, data); err != nil {
				klog.Errorf("send message error:%v", err)
				isConnected = false
			}
		}
	}
	ctx.EdgeSubscriber.Delete(objType, key)
}

func checkIsNeedToWatch(objType, fieldSelector, namespace, name string, obj runtime.Object) bool {
	var getNodeName string

	resource := reflect.ValueOf(obj).Elem()
	getNamespace := resource.FieldByName("ObjectMeta").FieldByName("Namespace")
	getName := resource.FieldByName("ObjectMeta").FieldByName("Name")
	if objType == util.ResourcePod {
		getNodeName = resource.FieldByName("Spec").FieldByName("NodeName").String()
	}

	// when k3s agent watches kubernetes endpoint, edgehub is no need to send the changed endpoint.
	if fieldSelector == "metadata.name=kubernetes" && objType == util.ResourceEndpoint {
		return false
	}

	// check if the resource is the watching one by the query params, if not, then it is no need to watch it.
	if !IsFilterFromQueryParams(fieldSelector, getNodeName, getName.String()) {
		return false
	}

	switch objType {
	case util.ResourceNode:
		if name != "" && getName.String() != name {
			return false
		}
	default:
		if (namespace != "" && getNamespace.String() != namespace) || (name != "" && getName.String() != name) {
			return false
		}
	}

	return true
}
