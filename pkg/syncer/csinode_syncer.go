package syncer

import (
	"fmt"

	"k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/util"
)

// CSINodeSyncer is responsible for synchronizing csinode from apiserver.
type CSINodeSyncer struct {
	ctx      *SyncContext
	Informer cache.SharedIndexInformer
}

func NewCSINodeSyncer(ctx *SyncContext) Syncer {
	informer, ok := ctx.InformerFactory[DefaultInformerFatory]
	if !ok {
		return nil
	}

	return &CSINodeSyncer{
		ctx:      ctx,
		Informer: informer.Storage().V1().CSINodes().Informer(),
	}
}

func (cs *CSINodeSyncer) startSyncer() error {
	if !cs.ctx.IsValid() {
		return fmt.Errorf("start csinode syncer failed: SyncContext is invalid")
	}

	registerInformerHandler(cs)

	return nil
}

// handleAddEvent puts the added csinode into persistent storage, and return to edge node as a watch event.
func (cs *CSINodeSyncer) handleAddEvent(obj interface{}) {
	csinode := obj.(*v1.CSINode)
	cs.addKindAndVersion(csinode)
	klog.V(4).Infof("add csinode: %s", csinode.Name)

	go syncToNode(watch.Added, util.ResourceCSINode, csinode)

	syncToStorage(cs.ctx, watch.Added, util.ResourceCSINode, csinode)
}

// handleUpdateEvent puts the modified csinode into persistent storage, and return to edge node as a watch event.
func (cs *CSINodeSyncer) handleUpdateEvent(old, new interface{}) {
	newCSINode := new.(*v1.CSINode)
	oldCSINode := old.(*v1.CSINode)
	if newCSINode.ResourceVersion == oldCSINode.ResourceVersion {
		// Periodic resync will send update events for all known Deployments.
		// Two different versions of the same Deployment will always have different RVs.
		return
	}

	cs.addKindAndVersion(newCSINode)
	klog.V(4).Infof("update csinode: %s", newCSINode.Name)

	go syncToNode(watch.Modified, util.ResourceCSINode, newCSINode)

	syncToStorage(cs.ctx, watch.Modified, util.ResourceCSINode, newCSINode)
}

// handleDeleteEvent delete the csinode from persistent storage, and return to edge node as a watch event.
func (cs *CSINodeSyncer) handleDeleteEvent(obj interface{}) {
	csinode, ok := obj.(*v1.CSINode)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.Errorf("Couldn't get object from tombstone %v", obj)
			return
		}
		csinode, ok = tombstone.Obj.(*v1.CSINode)
		if !ok {
			klog.Errorf("Tombstone contained object that is not a csinode %v", obj)
			return
		}
	}

	cs.addKindAndVersion(csinode)
	klog.V(4).Infof("delete csinode: %s", csinode.Name)

	go syncToNode(watch.Deleted, util.ResourceCSINode, csinode)

	syncToStorage(cs.ctx, watch.Deleted, util.ResourceCSINode, csinode)
}

// getInformer returns informer of this syncer.
func (cs *CSINodeSyncer) getInformer() cache.SharedIndexInformer {
	return cs.Informer
}

func (cs *CSINodeSyncer) addKindAndVersion(csinode *v1.CSINode) {
	csinode.APIVersion = "storage.k8s.io/v1"
	csinode.Kind = "CSINode"
}
