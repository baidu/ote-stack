module github.com/baidu/ote-stack

go 1.12

require (
	github.com/emicklei/go-restful v2.9.5+incompatible
	github.com/evanphx/json-patch v4.2.0+incompatible
	github.com/golang/protobuf v1.3.2
	github.com/gorilla/mux v1.7.2
	github.com/gorilla/websocket v1.4.0
	github.com/pkg/errors v0.9.1
	github.com/rancher/dynamiclistener v0.2.0
	github.com/segmentio/ksuid v1.0.2
	github.com/spf13/cobra v0.0.4
	github.com/stretchr/testify v1.4.0
	github.com/syndtr/goleveldb v1.0.0
	golang.org/x/net v0.7.0
	k8s.io/api v0.18.4
	k8s.io/apimachinery v0.18.4
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/code-generator v0.17.4
	k8s.io/component-base v0.0.0-20190602130718-4ec519775454
	k8s.io/gengo v0.0.0-20191010091904-7fa3014cb28f // indirect
	k8s.io/klog v1.0.0
)

replace (
	k8s.io/api => github.com/rancher/kubernetes/staging/src/k8s.io/api v1.17.4-k3s1
	k8s.io/apimachinery => github.com/rancher/kubernetes/staging/src/k8s.io/apimachinery v1.17.4-k3s1
	k8s.io/client-go => github.com/rancher/kubernetes/staging/src/k8s.io/client-go v1.17.4-k3s1
	k8s.io/component-base => github.com/rancher/kubernetes/staging/src/k8s.io/component-base v1.17.4-k3s1
)
