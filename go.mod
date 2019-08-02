module github.com/baidu/ote-stack

go 1.12

require (
	github.com/evanphx/json-patch v4.1.0+incompatible // indirect
	github.com/gogo/protobuf v1.2.1 // indirect
	github.com/golang/groupcache v0.0.0-20180924190550-6f2cf27854a4 // indirect
	github.com/golang/protobuf v1.3.2
	github.com/googleapis/gnostic v0.1.0 // indirect
	github.com/gorilla/mux v1.7.2
	github.com/gorilla/websocket v1.4.0
	github.com/hashicorp/golang-lru v0.5.1 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/kr/pretty v0.1.0 // indirect
	github.com/onsi/ginkgo v1.7.0 // indirect
	github.com/onsi/gomega v1.4.3 // indirect
	github.com/spf13/cobra v0.0.4
	github.com/stretchr/testify v1.3.0
	golang.org/x/crypto v0.0.0-20190424203555-c05e17bb3b2d // indirect
	golang.org/x/net v0.0.0-20190603091049-60506f45cf65
	golang.org/x/sys v0.0.0-20190425145619-16072639606e // indirect
	golang.org/x/text v0.3.1 // indirect
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	golang.org/x/tools v0.0.0-20190606050223-4d9ae51c2468 // indirect
	google.golang.org/genproto v0.0.0-20190307195333-5fe7a883aa19 // indirect
	google.golang.org/grpc v1.21.1
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
	k8s.io/api v0.0.0-20190612210016-7525909cc6da
	k8s.io/apimachinery v0.0.0-20190612205821-1799e75a0719
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/code-generator v0.0.0-20190531131525-17d711082421
	k8s.io/component-base v0.0.0-20190602130718-4ec519775454
	k8s.io/gengo v0.0.0-20190327210449-e17681d19d3a // indirect
	k8s.io/klog v0.3.2
	k8s.io/kube-openapi v0.0.0-20190603182131-db7b694dc208 // indirect
	k8s.io/utils v0.0.0-20190308190857-21c4ce38f2a7 // indirect
)

replace k8s.io/client-go v11.0.0+incompatible => github.com/kubernetes/client-go v0.0.0-20190612210332-e4cdb82809fc
