package syncer

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/util"
)

// NamespaceSyncer is responsible for synchronizing namespace from apiserver.
type NamespaceSyncer struct {
	ctx      *SyncContext
	Informer cache.SharedIndexInformer
}

func NewNamespaceSyncer(ctx *SyncContext) Syncer {
	informer, ok := ctx.InformerFactory[DefaultInformerFatory]
	if !ok {
		return nil
	}

	return &NamespaceSyncer{
		ctx:      ctx,
		Informer: informer.Core().V1().Namespaces().Informer(),
	}
}

func (ns *NamespaceSyncer) startSyncer() error {
	if !ns.ctx.IsValid() {
		return fmt.Errorf("start namespace syncer failed: SyncContext is invalid")
	}

	registerInformerHandler(ns)

	return nil
}

// handleAddEvent puts the added namespace into persistent storage, and return to edge node as a watch event.
func (ns *NamespaceSyncer) handleAddEvent(obj interface{}) {
	namespace := obj.(*corev1.Namespace)
	ns.addKindAndVersion(namespace)
	klog.V(4).Infof("add namespace: %s", namespace.Name)

	go syncToNode(watch.Added, util.ResourceNamespace, namespace)

	syncToStorage(ns.ctx, watch.Added, util.ResourceNamespace, namespace)
}

// handleUpdateEvent puts the modified namespace into persistent storage, and return to edge node as a watch event.
func (ns *NamespaceSyncer) handleUpdateEvent(old, new interface{}) {
	newNamespace := new.(*corev1.Namespace)
	oldNamespace := old.(*corev1.Namespace)
	if newNamespace.ResourceVersion == oldNamespace.ResourceVersion {
		// Periodic resync will send update events for all known Deployments.
		// Two different versions of the same Deployment will always have different RVs.
		return
	}

	ns.addKindAndVersion(newNamespace)
	klog.V(4).Infof("update namespace: %s", newNamespace.Name)

	go syncToNode(watch.Modified, util.ResourceNamespace, newNamespace)

	syncToStorage(ns.ctx, watch.Modified, util.ResourceNamespace, newNamespace)
}

// handleDeleteEvent delete the namespace from persistent storage, and return to edge node as a watch event.
func (ns *NamespaceSyncer) handleDeleteEvent(obj interface{}) {
	namespace, ok := obj.(*corev1.Namespace)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.Errorf("Couldn't get object from tombstone %v", obj)
			return
		}
		namespace, ok = tombstone.Obj.(*corev1.Namespace)
		if !ok {
			klog.Errorf("Tombstone contained object that is not a namespace %v", obj)
			return
		}
	}

	ns.addKindAndVersion(namespace)
	klog.V(4).Infof("delete namespace: %s", namespace.Name)

	go syncToNode(watch.Deleted, util.ResourceNamespace, namespace)

	syncToStorage(ns.ctx, watch.Deleted, util.ResourceNamespace, namespace)
}

// getInformer returns informer of this syncer.
func (ns *NamespaceSyncer) getInformer() cache.SharedIndexInformer {
	return ns.Informer
}

func (ns *NamespaceSyncer) addKindAndVersion(namespace *corev1.Namespace) {
	namespace.APIVersion = "v1"
	namespace.Kind = "Namespace"
}
