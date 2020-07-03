package handler

import (
	"regexp"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/loadbalancer"
	"github.com/baidu/ote-stack/pkg/storage"
	"github.com/baidu/ote-stack/pkg/syncer"
)

var connectionUpgradeRegex = regexp.MustCompile("(^|.*,\\s*)upgrade($|\\s*,)")

type Response struct {
	Code int
	Err  error
	Body []byte
}

type HandlerContext struct {
	Store          *storage.EdgehubStorage
	Lb             *loadbalancer.LoadBalancer
	K8sClient      kubernetes.Interface
	EdgeSubscriber *syncer.ResourceSubscriber
}

type ServiceHandler struct {
	Ctx           *HandlerContext
	ServerHandler map[string]Handler
}

type Handler interface {
	Create([]byte, string) *Response
	Delete(string, string, []byte) *Response
	List(string, string, string) *Response
	Get(string) *Response
	UpdatePut(string, string, []byte) *Response
	UpdatePatch(string, string, []byte) *Response
	GetInitWatchEvent(string, string, string) []metav1.WatchEvent
}

func IsFilterFromQueryParams(fieldSelector, nodeName, name string) bool {
	if fieldSelector != "" {
		result := strings.Split(fieldSelector, "=")
		if len(result) < 2 {
			return true
		}

		switch result[0] {
		case syncer.ObjectNameField:
			if result[1] != name {
				return false
			}
		case syncer.SpecNodeNameField:
			if result[1] != nodeName {
				return false
			}
		}
	}

	return true
}

func (h *HandlerContext) IsValid() bool {
	if h == nil {
		klog.Errorf("handler context is nil")
		return false
	}

	if h.Store == nil {
		klog.Errorf("handlerContext's storage is nil")
		return false
	}

	if h.K8sClient == nil {
		klog.Errorf("handlerContext's K8sClient is nil")
		return false
	}

	if h.Lb == nil {
		klog.Errorf("handlerContext's lb is nil")
		return false
	}

	if h.EdgeSubscriber == nil {
		klog.Errorf("handlerContext's EdgeSubscriber is nil")
		return false
	}

	return true
}

// CheckResponseCode check if response code is valid.Response Code should be three digits.
func CheckResponseCode(code int) bool {
	if code < 100 || code > 999 {
		return false
	}

	return true
}
