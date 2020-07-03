package syncer

import (
	"fmt"

	"k8s.io/api/coordination/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/util"
)

// LeaseSyncer is responsible for synchronizing nodelease from apiserver.
type LeaseSyncer struct {
	ctx      *SyncContext
	Informer cache.SharedIndexInformer
}

func NewLeaseSyncer(ctx *SyncContext) Syncer {
	informer, ok := ctx.InformerFactory[NodeInformerFactory]
	if !ok {
		return nil
	}

	return &LeaseSyncer{
		ctx:      ctx,
		Informer: informer.Coordination().V1().Leases().Informer(),
	}
}

func (ls *LeaseSyncer) startSyncer() error {
	if !ls.ctx.IsValid() {
		return fmt.Errorf("start lease syncer failed: SyncContext is invalid")
	}

	registerInformerHandler(ls)

	return nil
}

// handleAddEvent puts the added lease into persistent storage, and return to edge node as a watch event.
func (ls *LeaseSyncer) handleAddEvent(obj interface{}) {
	lease := obj.(*v1.Lease)
	ls.addKindAndVersion(lease)
	klog.V(4).Infof("add lease: %s", lease.Name)

	go syncToNode(watch.Added, util.ResourceNodeLease, lease)

	syncToStorage(ls.ctx, watch.Added, util.ResourceNodeLease, lease)
}

// handleUpdateEvent puts the modified lease into persistent storage, and return to edge node as a watch event.
func (ls *LeaseSyncer) handleUpdateEvent(old, new interface{}) {
	newLease := new.(*v1.Lease)
	oldLease := old.(*v1.Lease)
	if newLease.ResourceVersion == oldLease.ResourceVersion {
		// Periodic resync will send update events for all known Deployments.
		// Two different versions of the same Deployment will always have different RVs.
		return
	}

	ls.addKindAndVersion(newLease)
	klog.V(5).Infof("update lease: %s", newLease.Name)

	go syncToNode(watch.Modified, util.ResourceNodeLease, newLease)

	syncToStorage(ls.ctx, watch.Modified, util.ResourceNodeLease, newLease)
}

// handleDeleteEvent delete the lease from persistent storage, and return to edge node as a watch event.
func (ls *LeaseSyncer) handleDeleteEvent(obj interface{}) {
	lease, ok := obj.(*v1.Lease)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.Errorf("Couldn't get object from tombstone %v", obj)
			return
		}
		lease, ok = tombstone.Obj.(*v1.Lease)
		if !ok {
			klog.Errorf("Tombstone contained object that is not a lease %v", obj)
			return
		}
	}

	ls.addKindAndVersion(lease)
	klog.V(4).Infof("delete lease: %s", lease.Name)

	go syncToNode(watch.Deleted, util.ResourceNodeLease, lease)

	syncToStorage(ls.ctx, watch.Deleted, util.ResourceNodeLease, lease)
}

// getInformer returns informer of this syncer.
func (ls *LeaseSyncer) getInformer() cache.SharedIndexInformer {
	return ls.Informer
}

func (ls *LeaseSyncer) addKindAndVersion(lease *v1.Lease) {
	lease.APIVersion = "coordination.k8s.io/v1"
	lease.Kind = "Lease"
}
