package resource

import (
	"net/http"

	"k8s.io/api/certificates/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/server/handler"
	"github.com/baidu/ote-stack/pkg/util"
)

type CertSigningRequestHandler struct {
	ctx *handler.HandlerContext
}

func NewCertSigningRequestHandler(ctx *handler.HandlerContext) handler.Handler {
	return &CertSigningRequestHandler{
		ctx: ctx,
	}
}

func (ch *CertSigningRequestHandler) Create(body []byte, uri string) *handler.Response {
	csr, err := util.GetObjectFromSerializeData(util.ResourceCSR, body)
	if err != nil {
		klog.Errorf("get object from serialized data failed: %v", err)
		return &handler.Response{
			Code: http.StatusBadRequest,
			Err:  err,
		}
	}

	key, err := cache.MetaNamespaceKeyFunc(csr)
	if err != nil {
		klog.Errorf("get csr key failed: %v", err)
		return &handler.Response{
			Code: http.StatusBadRequest,
			Err:  err,
		}
	}

	return handler.DoCreate(ch.ctx, util.ResourceCSR, key, uri, body)
}

func (ch *CertSigningRequestHandler) Delete(key, uri string, body []byte) *handler.Response {
	return handler.DoDelete(ch.ctx, util.ResourceCSR, key, uri, body)
}

func (ch *CertSigningRequestHandler) List(fieldSelector, namespace, uri string) *handler.Response {
	result := v1beta1.CertificateSigningRequestList{}
	result.Kind = "CertificateSigningRequestList"
	result.APIVersion = "certificates.k8s.io/v1beta1"
	result.SelfLink = uri

	list := ch.ctx.Store.List(util.ResourceCSR)

	for _, item := range list {
		csr := item.(*v1beta1.CertificateSigningRequest)

		if namespace != "" && csr.Namespace != namespace {
			continue
		}

		if !handler.IsFilterFromQueryParams(fieldSelector, "", csr.Name) {
			klog.V(4).Infof("check FilterFromQueryParams failed")
			continue
		}

		result.Items = append(result.Items, *csr)
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

func (ch *CertSigningRequestHandler) Get(key string) *handler.Response {
	return handler.DoGet(ch.ctx, util.ResourceCSR, key)
}

func (ch *CertSigningRequestHandler) UpdatePut(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePut(ch.ctx, util.ResourceCSR, key, uri, body)
}

func (ch *CertSigningRequestHandler) UpdatePatch(key, uri string, body []byte) *handler.Response {
	return handler.DoUpdatePatch(ch.ctx, util.ResourceCSR, key, uri, body)
}

func (ch *CertSigningRequestHandler) GetInitWatchEvent(fieldSelector, namespace, name string) []metav1.WatchEvent {
	var result []metav1.WatchEvent
	list := ch.ctx.Store.List(util.ResourceCSR)

	for _, item := range list {
		csr := item.(*v1beta1.CertificateSigningRequest)

		if namespace != "" && csr.Namespace != namespace {
			continue
		}

		if name != "" && csr.Name != name {
			continue
		}

		if !handler.IsFilterFromQueryParams(fieldSelector, "", csr.Name) {
			klog.V(4).Infof("check FilterFromQueryParams failed")
			continue
		}

		event := metav1.WatchEvent{
			Type:   string(watch.Added),
			Object: runtime.RawExtension{Object: csr},
		}
		result = append(result, event)
	}

	return result
}
