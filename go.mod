module github.com/baidu/ote-stack

go 1.12

require (
	github.com/Azure/go-autorest/autorest v0.2.0 // indirect
	github.com/appscode/jsonpatch v0.0.0-20190108182946-7c0e3b262f30 // indirect
	github.com/coreos/prometheus-operator v0.30.1 // indirect
	github.com/go-logr/logr v0.1.0 // indirect
	github.com/go-logr/zapr v0.1.1 // indirect
	github.com/go-openapi/spec v0.19.0 // indirect
	github.com/go-yaml/yaml v2.1.0+incompatible // indirect
	github.com/golang/protobuf v1.3.1
	github.com/gophercloud/gophercloud v0.1.0 // indirect
	github.com/gorilla/mux v1.7.2
	github.com/gorilla/websocket v1.4.0
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/operator-framework/operator-sdk v0.8.1 // indirect
	github.com/spf13/cobra v0.0.4
	github.com/stretchr/testify v1.3.0
	github.com/tidwall/gjson v1.2.1 // indirect
	github.com/tidwall/match v1.0.1 // indirect
	github.com/tidwall/pretty v1.0.0 // indirect
	go.uber.org/atomic v1.4.0 // indirect
	go.uber.org/multierr v1.1.0 // indirect
	go.uber.org/zap v1.10.0 // indirect
	golang.org/x/net v0.0.0-20190603091049-60506f45cf65
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	google.golang.org/grpc v1.21.1
	k8s.io/api v0.0.0-20190612210016-7525909cc6da
	k8s.io/apimachinery v0.0.0-20190612205821-1799e75a0719
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/code-generator v0.0.0-20190531131525-17d711082421
	k8s.io/component-base v0.0.0-20190602130718-4ec519775454
	k8s.io/klog v0.3.2
	k8s.io/kube-openapi v0.0.0-20190603182131-db7b694dc208 // indirect
	sigs.k8s.io/controller-runtime v0.1.11 // indirect
)

replace k8s.io/client-go v11.0.0+incompatible => github.com/kubernetes/client-go v0.0.0-20190612210332-e4cdb82809fc
