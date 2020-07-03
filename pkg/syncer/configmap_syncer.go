package syncer

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/util"
)

// ConfigMapSyncer is responsible for synchronizing configmap from apiserver.
type ConfigMapSyncer struct {
	ctx      *SyncContext
	Informer cache.SharedIndexInformer
}

func NewConfigMapSyncer(ctx *SyncContext) Syncer {
	informer, ok := ctx.InformerFactory[DefaultInformerFatory]
	if !ok {
		return nil
	}

	return &ConfigMapSyncer{
		ctx:      ctx,
		Informer: informer.Core().V1().ConfigMaps().Informer(),
	}
}

func (cs *ConfigMapSyncer) startSyncer() error {
	if !cs.ctx.IsValid() {
		return fmt.Errorf("start configmap syncer failed: SyncContext is invalid")
	}

	registerInformerHandler(cs)

	return nil
}

// handleAddEvent puts the added configmap into persistent storage, and return to edge node as a watch event.
func (cs *ConfigMapSyncer) handleAddEvent(obj interface{}) {
	configmap := obj.(*corev1.ConfigMap)
	cs.addKindAndVersion(configmap)
	klog.V(4).Infof("add configmap: %s", configmap.Name)

	go syncToNode(watch.Added, util.ResourceConfigMap, configmap)

	syncToStorage(cs.ctx, watch.Added, util.ResourceConfigMap, configmap)
}

// handleUpdateEvent puts the modified configmap into persistent storage, and return to edge node as a watch event.
func (cs *ConfigMapSyncer) handleUpdateEvent(old, new interface{}) {
	newConfigMap := new.(*corev1.ConfigMap)
	oldConfigMap := old.(*corev1.ConfigMap)
	if newConfigMap.ResourceVersion == oldConfigMap.ResourceVersion {
		// Periodic resync will send update events for all known Deployments.
		// Two different versions of the same Deployment will always have different RVs.
		return
	}

	cs.addKindAndVersion(newConfigMap)
	klog.V(4).Infof("update configmap: %s", newConfigMap.Name)

	go syncToNode(watch.Modified, util.ResourceConfigMap, newConfigMap)

	syncToStorage(cs.ctx, watch.Modified, util.ResourceConfigMap, newConfigMap)
}

// handleDeleteEvent delete the configmap from persistent storage, and return to edge node as a watch event.
func (cs *ConfigMapSyncer) handleDeleteEvent(obj interface{}) {
	configmap, ok := obj.(*corev1.ConfigMap)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.Errorf("Couldn't get object from tombstone %v", obj)
			return
		}
		configmap, ok = tombstone.Obj.(*corev1.ConfigMap)
		if !ok {
			klog.Errorf("Tombstone contained object that is not a configmap %v", obj)
			return
		}
	}

	cs.addKindAndVersion(configmap)
	klog.V(4).Infof("delete configmap: %s", configmap.Name)

	go syncToNode(watch.Deleted, util.ResourceConfigMap, configmap)

	syncToStorage(cs.ctx, watch.Deleted, util.ResourceConfigMap, configmap)
}

// getInformer returns informer of this syncer.
func (cs *ConfigMapSyncer) getInformer() cache.SharedIndexInformer {
	return cs.Informer
}

func (cs *ConfigMapSyncer) addKindAndVersion(configmap *corev1.ConfigMap) {
	configmap.APIVersion = "v1"
	configmap.Kind = "ConfigMap"
}
