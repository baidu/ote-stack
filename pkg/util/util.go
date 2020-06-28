package util

import (
	"fmt"

	"github.com/segmentio/ksuid"
	authv1 "k8s.io/api/authentication/v1"
	authorv1 "k8s.io/api/authorization/v1"
	"k8s.io/api/certificates/v1beta1"
	"k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	nodev1beta1 "k8s.io/api/node/v1beta1"
	storagev1 "k8s.io/api/storage/v1"
	storagev1beta1 "k8s.io/api/storage/v1beta1"
	"k8s.io/apimachinery/pkg/util/json"
)

const (
	SplitByte = "/"

	ResourcePod                 = "pods"
	ResourceNode                = "nodes"
	ResourceNodeLease           = "leases"
	ResourceEndpoint            = "endpoints"
	ResourceService             = "services"
	ResourceConfigMap           = "configmaps"
	ResourceSecret              = "secrets"
	ResourceEvent               = "events"
	ResourceNamespace           = "namespaces"
	ResourceNetworkPolicy       = "networkpolicies"
	ResourceCSIDriver           = "csidrivers"
	ResourceCSINode             = "csinodes"
	ResourceRuntimeClass        = "runtimeclasses"
	ResourceTokenReview         = "tokenreviews"
	ResourceSubjectAccessReview = "subjectaccessreviews"
	ResourceCSR                 = "certificatesigningrequests"
)

func FormKeyName(namespace, name string) string {
	if namespace == "" {
		return name
	}

	return namespace + SplitByte + name
}

func GetUniqueId() string {
	return ksuid.New().String()
}

func GetObjectFromSerializeData(objType string, data []byte) (interface{}, error) {
	switch objType {
	case ResourcePod:
		pod := &corev1.Pod{}
		err := json.Unmarshal(data, pod)
		if err != nil {
			return nil, fmt.Errorf("json unmarshal pod failed: %v", err)
		}
		return pod, nil
	case ResourceNode:
		node := &corev1.Node{}
		err := json.Unmarshal(data, node)
		if err != nil {
			return nil, fmt.Errorf("json unmarshal node failed: %v", err)
		}
		return node, nil
	case ResourceNodeLease:
		lease := &v1.Lease{}
		err := json.Unmarshal(data, lease)
		if err != nil {
			return nil, fmt.Errorf("json unmarshal lease failed: %v", err)
		}
		return lease, nil
	case ResourceEndpoint:
		endpoint := &corev1.Endpoints{}
		err := json.Unmarshal(data, endpoint)
		if err != nil {
			return nil, fmt.Errorf("json unmarshal endpoint failed: %v", err)
		}
		return endpoint, nil
	case ResourceService:
		service := &corev1.Service{}
		err := json.Unmarshal(data, service)
		if err != nil {
			return nil, fmt.Errorf("json unmarshal service failed: %v", err)
		}
		return service, nil
	case ResourceConfigMap:
		configmap := &corev1.ConfigMap{}
		err := json.Unmarshal(data, configmap)
		if err != nil {
			return nil, fmt.Errorf("json unmarshal configmap failed: %v", err)
		}
		return configmap, nil
	case ResourceSecret:
		secret := &corev1.Secret{}
		err := json.Unmarshal(data, secret)
		if err != nil {
			return nil, fmt.Errorf("json unmarshal secret failed: %v", err)
		}
		return secret, nil
	case ResourceEvent:
		event := &corev1.Event{}
		err := json.Unmarshal(data, event)
		if err != nil {
			return nil, fmt.Errorf("json unmarshal event failed: %v", err)
		}
		return event, nil
	case ResourceNamespace:
		namespace := &corev1.Namespace{}
		err := json.Unmarshal(data, namespace)
		if err != nil {
			return nil, fmt.Errorf("json unmarshal namespace failed: %v", err)
		}
		return namespace, nil
	case ResourceNetworkPolicy:
		networkpolicy := &netv1.NetworkPolicy{}
		err := json.Unmarshal(data, networkpolicy)
		if err != nil {
			return nil, fmt.Errorf("json unmarshal networkpolicy failed: %v", err)
		}
		return networkpolicy, nil
	case ResourceCSIDriver:
		csidriver := &storagev1beta1.CSIDriver{}
		err := json.Unmarshal(data, csidriver)
		if err != nil {
			return nil, fmt.Errorf("json unmarshal csidriver failed: %v", err)
		}
		return csidriver, nil
	case ResourceCSINode:
		csinode := &storagev1.CSINode{}
		err := json.Unmarshal(data, csinode)
		if err != nil {
			return nil, fmt.Errorf("json unmarshal csinode failed: %v", err)
		}
		return csinode, nil
	case ResourceRuntimeClass:
		runtimeclass := &nodev1beta1.RuntimeClass{}
		err := json.Unmarshal(data, runtimeclass)
		if err != nil {
			return nil, fmt.Errorf("json unmarshal runtimeclass failed: %v", err)
		}
		return runtimeclass, nil
	case ResourceTokenReview:
		tokenreview := &authv1.TokenReview{}
		err := json.Unmarshal(data, tokenreview)
		if err != nil {
			return nil, fmt.Errorf("json unmarshal tokenreview failed: %v", err)
		}
		return tokenreview, nil
	case ResourceSubjectAccessReview:
		subjectAccessReview := &authorv1.SubjectAccessReview{}
		err := json.Unmarshal(data, subjectAccessReview)
		if err != nil {
			return nil, fmt.Errorf("json unmarshal subjectAccessReview failed: %v", err)
		}
		return subjectAccessReview, nil
	case ResourceCSR:
		csr := &v1beta1.CertificateSigningRequest{}
		err := json.Unmarshal(data, csr)
		if err != nil {
			return nil, fmt.Errorf("json unmarshal csr failed: %v", err)
		}
		return csr, nil
	default:
		return nil, fmt.Errorf("unsupported resource type: %s", objType)
	}
}
