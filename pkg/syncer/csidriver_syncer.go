package syncer

import (
	"fmt"

	"k8s.io/api/storage/v1beta1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/util"
)

// CSIDriverSyncer is responsible for synchronizing csidriver from apiserver.
type CSIDriverSyncer struct {
	ctx      *SyncContext
	Informer cache.SharedIndexInformer
}

func NewCSIDriverSyncer(ctx *SyncContext) Syncer {
	informer, ok := ctx.InformerFactory[DefaultInformerFatory]
	if !ok {
		return nil
	}

	return &CSIDriverSyncer{
		ctx:      ctx,
		Informer: informer.Storage().V1beta1().CSIDrivers().Informer(),
	}
}

func (cs *CSIDriverSyncer) startSyncer() error {
	if !cs.ctx.IsValid() {
		return fmt.Errorf("start csidriver syncer failed: SyncContext is invalid")
	}

	registerInformerHandler(cs)

	return nil
}

// handleAddEvent puts the added csidriver into persistent storage, and return to edge node as a watch event.
func (cs *CSIDriverSyncer) handleAddEvent(obj interface{}) {
	csidriver := obj.(*v1beta1.CSIDriver)
	cs.addKindAndVersion(csidriver)

	klog.V(4).Infof("add csidriver: %s", csidriver.Name)

	go syncToNode(watch.Added, util.ResourceCSIDriver, csidriver)

	syncToStorage(cs.ctx, watch.Added, util.ResourceCSIDriver, csidriver)
}

// handleUpdateEvent puts the modified csidriver into persistent storage, and return to edge node as a watch event.
func (cs *CSIDriverSyncer) handleUpdateEvent(old, new interface{}) {
	newCSIDriver := new.(*v1beta1.CSIDriver)
	oldCSIDriver := old.(*v1beta1.CSIDriver)
	if newCSIDriver.ResourceVersion == oldCSIDriver.ResourceVersion {
		// Periodic resync will send update events for all known Deployments.
		// Two different versions of the same Deployment will always have different RVs.
		return
	}

	cs.addKindAndVersion(newCSIDriver)
	klog.V(4).Infof("update csidriver: %s", newCSIDriver.Name)

	go syncToNode(watch.Modified, util.ResourceCSIDriver, newCSIDriver)

	syncToStorage(cs.ctx, watch.Modified, util.ResourceCSIDriver, newCSIDriver)
}

// handleDeleteEvent delete the csidriver from persistent storage, and return to edge node as a watch event.
func (cs *CSIDriverSyncer) handleDeleteEvent(obj interface{}) {
	csidriver, ok := obj.(*v1beta1.CSIDriver)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.Errorf("Couldn't get object from tombstone %v", obj)
			return
		}
		csidriver, ok = tombstone.Obj.(*v1beta1.CSIDriver)
		if !ok {
			klog.Errorf("Tombstone contained object that is not a csidriver %v", obj)
			return
		}
	}

	cs.addKindAndVersion(csidriver)
	klog.V(4).Infof("delete csidriver: %s", csidriver.Name)

	go syncToNode(watch.Deleted, util.ResourceCSIDriver, csidriver)

	syncToStorage(cs.ctx, watch.Deleted, util.ResourceCSIDriver, csidriver)
}

// getInformer returns informer of this syncer.
func (cs *CSIDriverSyncer) getInformer() cache.SharedIndexInformer {
	return cs.Informer
}

func (cs *CSIDriverSyncer) addKindAndVersion(csidriver *v1beta1.CSIDriver) {
	csidriver.APIVersion = "storage.k8s.io/v1beta1"
	csidriver.Kind = "CSIDriver"
}
