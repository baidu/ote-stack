package syncer

import (
	"fmt"

	"k8s.io/api/node/v1beta1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/util"
)

// RuntimeClassSyncer is responsible for synchronizing runtimeclass from apiserver.
type RuntimeClassSyncer struct {
	ctx      *SyncContext
	Informer cache.SharedIndexInformer
}

func NewRuntimeClassSyncer(ctx *SyncContext) Syncer {
	informer, ok := ctx.InformerFactory[DefaultInformerFatory]
	if !ok {
		return nil
	}

	return &RuntimeClassSyncer{
		ctx:      ctx,
		Informer: informer.Node().V1beta1().RuntimeClasses().Informer(),
	}
}

func (rs *RuntimeClassSyncer) startSyncer() error {
	if !rs.ctx.IsValid() {
		return fmt.Errorf("start runtimeclass syncer failed: SyncContext is invalid")
	}

	registerInformerHandler(rs)

	return nil
}

// handleAddEvent puts the added runtimeclass into persistent storage, and return to edge node as a watch event.
func (rs *RuntimeClassSyncer) handleAddEvent(obj interface{}) {
	runtimeclass := obj.(*v1beta1.RuntimeClass)
	rs.addKindAndVersion(runtimeclass)
	klog.V(4).Infof("add runtimeclass: %s", runtimeclass.Name)

	go syncToNode(watch.Added, util.ResourceRuntimeClass, runtimeclass)

	syncToStorage(rs.ctx, watch.Added, util.ResourceRuntimeClass, runtimeclass)
}

// handleUpdateEvent puts the modified runtimeclass into persistent storage, and return to edge node as a watch event.
func (rs *RuntimeClassSyncer) handleUpdateEvent(old, new interface{}) {
	newRuntimeCLass := new.(*v1beta1.RuntimeClass)
	oldRuntimeCLass := old.(*v1beta1.RuntimeClass)
	if newRuntimeCLass.ResourceVersion == oldRuntimeCLass.ResourceVersion {
		// Periodic resync will send update events for all known Deployments.
		// Two different versions of the same Deployment will always have different RVs.
		return
	}

	rs.addKindAndVersion(newRuntimeCLass)
	klog.V(4).Infof("update runtimeclass: %s", newRuntimeCLass.Name)

	go syncToNode(watch.Modified, util.ResourceRuntimeClass, newRuntimeCLass)

	syncToStorage(rs.ctx, watch.Modified, util.ResourceRuntimeClass, newRuntimeCLass)
}

// handleDeleteEvent delete the runtimeclass from persistent storage, and return to edge node as a watch event.
func (rs *RuntimeClassSyncer) handleDeleteEvent(obj interface{}) {
	runtimeclass, ok := obj.(*v1beta1.RuntimeClass)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.Errorf("Couldn't get object from tombstone %v", obj)
			return
		}
		runtimeclass, ok = tombstone.Obj.(*v1beta1.RuntimeClass)
		if !ok {
			klog.Errorf("Tombstone contained object that is not a runtimeclass %v", obj)
			return
		}
	}

	rs.addKindAndVersion(runtimeclass)
	klog.V(4).Infof("delete runtimeclass: %s", runtimeclass.Name)

	go syncToNode(watch.Deleted, util.ResourceRuntimeClass, runtimeclass)

	syncToStorage(rs.ctx, watch.Deleted, util.ResourceRuntimeClass, runtimeclass)
}

// getInformer returns informer of this syncer.
func (rs *RuntimeClassSyncer) getInformer() cache.SharedIndexInformer {
	return rs.Informer
}

func (rs *RuntimeClassSyncer) addKindAndVersion(runtimeclass *v1beta1.RuntimeClass) {
	runtimeclass.APIVersion = "node.k8s.io/v1beta1"
	runtimeclass.Kind = "RuntimeClass"
}
