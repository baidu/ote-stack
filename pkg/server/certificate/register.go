package certificate

import (
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/emicklei/go-restful"
	"github.com/gorilla/websocket"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/loadbalancer"
	"github.com/baidu/ote-stack/pkg/storage"
)

const (
	ServePath = "/"
)

var upgrader = websocket.Upgrader{}

type CertContext struct {
	DataPath  string
	CaFile    string
	TokenFile string
	ServerURL string
	certInfo  *Info

	Store     *storage.EdgehubStorage
	Lb        *loadbalancer.LoadBalancer
	K8sClient kubernetes.Interface
}

type CertHandler struct {
	Ctx *CertContext
}

func newCertHandler(ctx *CertContext) *CertHandler {
	return &CertHandler{
		Ctx: ctx,
	}
}

func NewAuthWebService(ctx *CertContext) *restful.WebService {
	c := newCertHandler(ctx)

	ws := new(restful.WebService)
	ws.Path(ServePath)
	ws.Route(ws.GET("cacerts").To(c.cacertsHandler))
	ws.Route(ws.GET("apis").To(c.apisHandler))
	ws.Route(ws.GET("v1-k3s/config").To(c.configHandler))
	ws.Route(ws.GET("v1-k3s/client-ca.crt").To(c.fileHandler))
	ws.Route(ws.GET("v1-k3s/server-ca.crt").To(c.fileHandler))
	ws.Route(ws.GET("v1-k3s/serving-kubelet.crt").To(c.keyHandler))
	ws.Route(ws.GET("v1-k3s/client-kubelet.crt").To(c.keyHandler))
	ws.Route(ws.GET("v1-k3s/client-kube-proxy.crt").To(c.fileHandler))
	ws.Route(ws.GET("v1-k3s/client-k3s-controller.crt").To(c.fileHandler))
	ws.Route(ws.GET("version").To(c.versionHandler))
	ws.Route(ws.GET("v1-k3s/connect").To(c.connectHandler))

	return ws
}

func (c *CertHandler) cacertsHandler(req *restful.Request, resp *restful.Response) {
	caFile, err := ioutil.ReadFile(c.Ctx.CaFile)
	if err != nil {
		klog.Errorf("get ca.pem failed: %v", err)
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	resp.Header().Set("content-type", "text/plain")
	resp.Write(caFile)
}

func (c *CertHandler) apisHandler(req *restful.Request, resp *restful.Response) {
	resp.WriteHeader(http.StatusOK)
}

func (c *CertHandler) configHandler(req *restful.Request, resp *restful.Response) {
	if !c.Ctx.Lb.IsRemoteEnable() {
		data, err := c.Ctx.Store.LevelDB.Get("cert", "config")
		if err != nil {
			klog.Errorf("get config failed: %v", err)
			resp.WriteError(http.StatusInternalServerError, err)
			return
		}
		resp.Write(data)
		return
	}

	tokenBytes, err := ioutil.ReadFile(c.Ctx.TokenFile)
	if err != nil {
		klog.Errorf("get node-token failed: %v", err)
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	token, err := NormalizeAndValidateTokenForUser(c.Ctx.ServerURL, strings.TrimSpace(string(tokenBytes)), "node")
	if err != nil {
		klog.Errorf("get token failed: %v", err)
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	info, err := ParseAndValidateToken(c.Ctx.ServerURL, token)
	if err != nil {
		klog.Errorf("get config failed: %v", err)
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}
	c.Ctx.certInfo = info

	data, err := getConfig(c.Ctx.certInfo)
	if err != nil {
		klog.Errorf("get config failed: %v", err)
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}
	resp.Write(data)

	// set into local storage
	if err := c.Ctx.Store.Update("cert", "config", data); err != nil {
		klog.Errorf("store k3s config failed")
	}
}

func (c *CertHandler) fileHandler(req *restful.Request, resp *restful.Response) {
	if !c.Ctx.Lb.IsRemoteEnable() {
		data, err := c.Ctx.Store.LevelDB.Get("cert", req.Request.RequestURI)
		if err != nil {
			klog.Errorf("get k3s %s failed: %v", req.Request.RequestURI, err)
			resp.WriteError(http.StatusInternalServerError, err)
			return
		}
		resp.Write(data)
		return
	}

	data, err := Get(req.Request.RequestURI, c.Ctx.certInfo)
	if err != nil {
		klog.Errorf("get cert file %s failed: %v", req.Request.RequestURI, err)
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	resp.Write(data)

	// set into local storage
	if err := c.Ctx.Store.Update("cert", req.Request.RequestURI, data); err != nil {
		klog.Errorf("store k3s %s failed", req.Request.RequestURI)
	}
}

func (c *CertHandler) keyHandler(req *restful.Request, resp *restful.Response) {
	if !c.Ctx.Lb.IsRemoteEnable() {
		data, err := c.Ctx.Store.LevelDB.Get("cert", req.Request.RequestURI)
		if err != nil {
			klog.Errorf("get k3s %s failed: %v", req.Request.RequestURI, err)
			resp.WriteError(http.StatusInternalServerError, err)
			return
		}
		resp.Write(data)
		return
	}

	nodePassword := req.Request.Header.Get("K3s-Node-Password")
	nodeName := req.Request.Header.Get("K3s-Node-Name")

	data, err := Request(req.Request.RequestURI, c.Ctx.certInfo, getNodeNamedCrt(nodeName, nodePassword))
	if err != nil {
		klog.Errorf("get key file %s failed: %v", req.Request.RequestURI, err)
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	resp.Write(data)

	// set into local storage
	if err := c.Ctx.Store.Update("cert", req.Request.RequestURI, data); err != nil {
		klog.Errorf("store k3s %s failed", req.Request.RequestURI)
	}
}

func (c *CertHandler) versionHandler(req *restful.Request, resp *restful.Response) {
	if !c.Ctx.Lb.IsRemoteEnable() {
		data, err := c.Ctx.Store.LevelDB.Get("cert", "version")
		if err != nil {
			klog.Errorf("get k3s version failed: %v", err)
			resp.WriteError(http.StatusInternalServerError, err)
			return
		}
		resp.Write(data)
		return
	}

	result := c.Ctx.K8sClient.Discovery().RESTClient().Get().AbsPath("/version").Do()
	raw, err := result.Raw()
	if err != nil {
		klog.Errorf("get version failed: %v", err)
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	resp.Write(raw)

	// set into local storage
	if err := c.Ctx.Store.Update("cert", "version", raw); err != nil {
		klog.Errorf("store k3s version failed")
	}
}

func (c *CertHandler) connectHandler(req *restful.Request, resp *restful.Response) {
	conn, err := upgrader.Upgrade(resp.ResponseWriter, req.Request, nil)
	if err != nil {
		klog.Errorf("upgrade connect failed: %v", err)
		return
	}

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			klog.Errorf("websocket read msg failed: %v", err)
			break
		}
	}

	conn.Close()
}

func (c *CertContext) IsValid() bool {
	if c == nil {
		klog.Errorf("cert context is nil")
		return false
	}

	if c.Store == nil {
		klog.Errorf("certContext's storage is nil")
		return false
	}

	if c.CaFile == "" {
		klog.Errorf("certContext's CaFile is nil")
		return false
	}

	if c.K8sClient == nil {
		klog.Errorf("certContext's K8sClient is nil")
		return false
	}

	if c.Lb == nil {
		klog.Errorf("certContext's lb is nil")
		return false
	}

	if c.DataPath == "" {
		klog.Errorf("certContext's data path is nil")
		return false
	}

	if c.ServerURL == "" {
		klog.Errorf("certContext's server url is nil")
		return false
	}

	if c.TokenFile == "" {
		klog.Errorf("certContext's token file is nil")
		return false
	}

	return true
}
