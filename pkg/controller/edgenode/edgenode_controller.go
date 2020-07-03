package edgenode

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	k8sinformer "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	otev1 "github.com/baidu/ote-stack/pkg/apis/ote/v1"
	ote "github.com/baidu/ote-stack/pkg/generated/clientset/versioned"
)

const (
	informerDuration    = 10 * time.Second
	updatedNodeDuration = 10 * time.Second

	EdgeControllerReady = "EdgeControllerReady"
	EdgeNodeNotReady    = "NotReady"
	EdgeNodeReady       = "Ready"
)

type ControllerContext struct {
	KubeClient kubernetes.Interface
	OteClient  ote.Interface
	StopChan   chan struct{}
}

type EdgeNodeController struct {
	ctx                *ControllerContext
	Informer           cache.SharedIndexInformer
	updateNodeMap      map[string]struct{}
	updateNodeMapMutex *sync.RWMutex
}

func NewEdgeNodeController(ctx *ControllerContext) *EdgeNodeController {
	return &EdgeNodeController{
		ctx:                ctx,
		updateNodeMap:      make(map[string]struct{}),
		updateNodeMapMutex: &sync.RWMutex{},
	}
}

func (ec *EdgeNodeController) Start() {
	kubeInformer := k8sinformer.NewSharedInformerFactory(ec.ctx.KubeClient, informerDuration)
	ec.Informer = kubeInformer.Core().V1().Nodes().Informer()

	ec.Informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: ec.handleNodeAdd,
		UpdateFunc: func(old, new interface{}) {
			newNode := new.(*corev1.Node)
			oldNode := old.(*corev1.Node)
			if newNode.ResourceVersion == oldNode.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				return
			}

			ec.handleNodeUpdate(newNode)
		},
		DeleteFunc: ec.handleNodeDelete,
	})

	kubeInformer.Start(ec.ctx.StopChan)

	go wait.Until(ec.updateNodeStatus, updatedNodeDuration, ec.ctx.StopChan)
}

func (ec *EdgeNodeController) handleNodeAdd(obj interface{}) {
	node := obj.(*corev1.Node)

	klog.Infof("add node: %s", node.Name)

	// if the new node's edge node resource is not exist, create it in k8s etcd.
	_, err := ec.ctx.OteClient.OteV1().EdgeNodes("kube-system").Get(node.Name, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		en := &otev1.EdgeNode{
			ObjectMeta: metav1.ObjectMeta{
				Name:      node.Name,
				Namespace: "kube-system",
			},
			Status: "NotReady",
		}

		_, err := ec.ctx.OteClient.OteV1().EdgeNodes("kube-system").Create(en)
		if err != nil {
			klog.Errorf("create new edge node failed: %v", err)
			return
		}
	} else if err != nil {
		klog.Errorf("get node from master failed: %v", err)
	}

	// if it is a NotReady node, should update edgenode's status, and add to updatenode map.
	_, condition := GetNodeCondition(&node.Status, corev1.NodeReady)
	if condition == nil {
		return
	}
	if condition.Status == corev1.ConditionUnknown || condition.Reason == EdgeControllerReady {
		// update edgenode status
		err := ec.updateEdgeNodeStatus(node.Name, EdgeNodeNotReady)
		if err != nil {
			klog.Errorf("edge node updated failed: %v", err)
		}

		// add to updatenode map
		ec.addUpdatedNode(node.Name)
	}
}

func (ec *EdgeNodeController) handleNodeUpdate(node *corev1.Node) {
	klog.Infof("update node: %s", node.Name)

	_, condition := GetNodeCondition(&node.Status, corev1.NodeReady)
	// if condition is null, it means the node is always not ready in edge, so edge controller don't need to update it.
	if condition == nil {
		// update edgenode status
		err := ec.updateEdgeNodeStatus(node.Name, EdgeNodeNotReady)
		if err != nil {
			klog.Errorf("edge node updated failed: %v", err)
		}

		return
	}

	// if condition is unknow, it means the node is disconnected from master, so edge controller need to update it.
	if condition.Status == corev1.ConditionUnknown {
		klog.Infof("node condition status is: %s", condition.Status)

		// update edgenode status
		err := ec.updateEdgeNodeStatus(node.Name, EdgeNodeNotReady)
		if err != nil {
			klog.Errorf("edge node updated failed: %v", err)
		}

		// add to node map
		ec.addUpdatedNode(node.Name)
		return
	}

	// if condition's reason is "EdgeControllerReady", it means it is always updated by edge controller, so no need to do anything.
	if condition.Reason == EdgeControllerReady {
		klog.Infof(" it is node controller to update status")
		return
	}

	// if node is ready, update edgenode status to "Ready".
	err := ec.updateEdgeNodeStatus(node.Name, EdgeNodeReady)
	if err != nil {
		klog.Errorf("edge node updated failed: %v", err)
	}

	ec.deleteUpdatedNode(node.Name)
}

func (ec *EdgeNodeController) handleNodeDelete(obj interface{}) {
	node := obj.(*corev1.Node)

	klog.Infof("delete node: %s", node.Name)

	// delete the edge node when the node is deleted from api-server.
	err := ec.ctx.OteClient.OteV1().EdgeNodes("kube-system").Delete(node.Name, metav1.NewDeleteOptions(0))
	if err != nil {
		klog.Errorf("delete edge node %s failed: %v", node.Name, err)
	}

	ec.deleteUpdatedNode(node.Name)
}

func (ec *EdgeNodeController) updateEdgeNodeStatus(nodeName string, desiredStatus string) error {
	edgeNode, err := ec.ctx.OteClient.OteV1().EdgeNodes("kube-system").Get(nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get edge node %s failed: %v", nodeName, err)
	}

	if edgeNode.Status == desiredStatus {
		return nil
	}

	edgeNode.Status = desiredStatus
	_, err = ec.ctx.OteClient.OteV1().EdgeNodes("kube-system").Update(edgeNode)
	if err != nil {
		return fmt.Errorf("update edge node %s failed: %v", nodeName, err)
	}

	return nil
}

func (ec *EdgeNodeController) addUpdatedNode(name string) {
	ec.updateNodeMapMutex.Lock()
	defer ec.updateNodeMapMutex.Unlock()

	if _, ok := ec.updateNodeMap[name]; !ok {
		ec.updateNodeMap[name] = struct{}{}
	}
}

func (ec *EdgeNodeController) deleteUpdatedNode(name string) {
	ec.updateNodeMapMutex.Lock()
	defer ec.updateNodeMapMutex.Unlock()

	if _, ok := ec.updateNodeMap[name]; !ok {
		return
	}

	delete(ec.updateNodeMap, name)
}

func (ec *EdgeNodeController) updateNodeStatus() {
	ec.updateNodeMapMutex.Lock()
	defer ec.updateNodeMapMutex.Unlock()

	for name, _ := range ec.updateNodeMap {
		data := getNewNodeConditionData()
		if data == nil {
			continue
		}

		_, err := ec.ctx.KubeClient.CoreV1().Nodes().PatchStatus(name, data)
		if err != nil {
			klog.Errorf("node %s patch failed: %v", name, err)
		}
	}
}

func getNewNodeConditionData() []byte {
	var ret = []corev1.NodeCondition{}

	condition := corev1.NodeCondition{}
	condition.Type = corev1.NodeMemoryPressure
	condition.Status = corev1.ConditionFalse
	condition.Reason = "KubeletHasSufficientMemory"
	condition.Message = "kubelet has sufficient memory available"
	condition.LastHeartbeatTime = metav1.Now()
	ret = append(ret, condition)

	condition.Type = corev1.NodeDiskPressure
	condition.Status = corev1.ConditionFalse
	condition.Reason = "KubeletHasNoDiskPressure"
	condition.Message = "kubelet has no disk pressure"
	condition.LastHeartbeatTime = metav1.Now()
	ret = append(ret, condition)

	condition.Type = corev1.NodePIDPressure
	condition.Status = corev1.ConditionFalse
	condition.Reason = "KubeletHasSufficientPID"
	condition.Message = "kubelet has sufficient PID available"
	condition.LastHeartbeatTime = metav1.Now()
	ret = append(ret, condition)

	condition.Type = corev1.NodeReady
	condition.Status = corev1.ConditionTrue
	condition.Reason = EdgeControllerReady
	condition.Message = "Edgecontroller is posting ready status"
	condition.LastHeartbeatTime = metav1.Now()
	ret = append(ret, condition)

	data := map[string]interface{}{
		"status": map[string][]corev1.NodeCondition{
			"conditions": ret,
		},
	}

	payload, err := json.Marshal(data)
	if err != nil {
		klog.Errorf("get node condition patch data failed: %v", err)
	}

	return payload
}

// GetNodeCondition extracts the provided condition from the given status and returns that.
// Returns nil and -1 if the condition is not present, and the index of the located condition.
func GetNodeCondition(status *corev1.NodeStatus, conditionType corev1.NodeConditionType) (int, *corev1.NodeCondition) {
	if status == nil {
		return -1, nil
	}
	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
			return i, &status.Conditions[i]
		}
	}
	return -1, nil
}
