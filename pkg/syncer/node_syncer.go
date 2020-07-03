package syncer

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/util"
)

// NodeSyncer is responsible for synchronizing node from apiserver.
type NodeSyncer struct {
	ctx      *SyncContext
	Informer cache.SharedIndexInformer
}

func NewNodeSyncer(ctx *SyncContext) Syncer {
	informer, ok := ctx.InformerFactory[NodeInformerFactory]
	if !ok {
		return nil
	}

	return &NodeSyncer{
		ctx:      ctx,
		Informer: informer.Core().V1().Nodes().Informer(),
	}
}

func (ns *NodeSyncer) startSyncer() error {
	if !ns.ctx.IsValid() {
		return fmt.Errorf("start node syncer failed: SyncContext is invalid")
	}

	registerInformerHandler(ns)

	return nil
}

// handleAddNode puts the added node into persistent storage, and return to edge node as a watch event.
func (ns *NodeSyncer) handleAddEvent(obj interface{}) {
	node := obj.(*corev1.Node)
	ns.addKindAndVersion(node)
	klog.V(4).Infof("add node: %s", node.Name)

	go syncToNode(watch.Added, util.ResourceNode, node)

	syncToStorage(ns.ctx, watch.Added, util.ResourceNode, node)
}

// handleUpdateEvent puts the modified node into persistent storage, and return to edge node as a watch event.
func (ns *NodeSyncer) handleUpdateEvent(old, new interface{}) {
	newNode := new.(*corev1.Node)
	oldNode := old.(*corev1.Node)
	if newNode.ResourceVersion == oldNode.ResourceVersion {
		// Periodic resync will send update events for all known Deployments.
		// Two different versions of the same Deployment will always have different RVs.
		return
	}

	ns.addKindAndVersion(newNode)
	klog.V(5).Infof("update node: %s", newNode.Name)

	go syncToNode(watch.Modified, util.ResourceNode, newNode)

	syncToStorage(ns.ctx, watch.Modified, util.ResourceNode, newNode)
}

// handleDeleteEvent delete the node from persistent storage, and return to edge node as a watch event.
func (ns *NodeSyncer) handleDeleteEvent(obj interface{}) {
	node, ok := obj.(*corev1.Node)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.Errorf("Couldn't get object from tombstone %v", obj)
			return
		}
		node, ok = tombstone.Obj.(*corev1.Node)
		if !ok {
			klog.Errorf("Tombstone contained object that is not a node %v", obj)
			return
		}
	}

	ns.addKindAndVersion(node)
	klog.V(4).Infof("delete node: %s", node.Name)

	go syncToNode(watch.Deleted, util.ResourceNode, node)

	syncToStorage(ns.ctx, watch.Deleted, util.ResourceNode, node)
}

// getInformer returns informer of this syncer.
func (ns *NodeSyncer) getInformer() cache.SharedIndexInformer {
	return ns.Informer
}

func (ns *NodeSyncer) addKindAndVersion(node *corev1.Node) {
	node.APIVersion = "v1"
	node.Kind = "Node"
}
