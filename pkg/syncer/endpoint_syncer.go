package syncer

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/util"
)

// EndpointSyncer is responsible for synchronizing endpoint from apiserver.
type EndpointSyncer struct {
	ctx      *SyncContext
	Informer cache.SharedIndexInformer
}

func NewEndpointSyncer(ctx *SyncContext) Syncer {
	informer, ok := ctx.InformerFactory[DefaultInformerFatory]
	if !ok {
		return nil
	}

	return &EndpointSyncer{
		ctx:      ctx,
		Informer: informer.Core().V1().Endpoints().Informer(),
	}
}

func (es *EndpointSyncer) startSyncer() error {
	if !es.ctx.IsValid() {
		return fmt.Errorf("start endpoint syncer failed: SyncContext is invalid")
	}

	registerInformerHandler(es)

	return nil
}

// handleAddEvent puts the added endpoint into persistent storage, and return to edge node as a watch event.
func (es *EndpointSyncer) handleAddEvent(obj interface{}) {
	endpoint := obj.(*corev1.Endpoints)
	es.addKindAndVersion(endpoint)

	klog.V(4).Infof("add endpoint: %s", endpoint.Name)

	go syncToNode(watch.Added, util.ResourceEndpoint, endpoint)

	syncToStorage(es.ctx, watch.Added, util.ResourceEndpoint, endpoint)
}

// handleUpdateEvent puts the modified ednpoint into persistent storage, and return to edge node as a watch event.
func (es *EndpointSyncer) handleUpdateEvent(old, new interface{}) {
	newEndpoint := new.(*corev1.Endpoints)
	oldEndpoint := old.(*corev1.Endpoints)
	if newEndpoint.ResourceVersion == oldEndpoint.ResourceVersion {
		// Periodic resync will send update events for all known Deployments.
		// Two different versions of the same Deployment will always have different RVs.
		return
	}

	es.addKindAndVersion(newEndpoint)
	klog.V(5).Infof("update endpoint: %s", newEndpoint.Name)

	go syncToNode(watch.Modified, util.ResourceEndpoint, newEndpoint)

	syncToStorage(es.ctx, watch.Modified, util.ResourceEndpoint, newEndpoint)
}

// handleDeleteEvent delete the endpoint from persistent storage, and return to edge node as a watch event.
func (es *EndpointSyncer) handleDeleteEvent(obj interface{}) {
	endpoint, ok := obj.(*corev1.Endpoints)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.Errorf("Couldn't get object from tombstone %v", obj)
			return
		}
		endpoint, ok = tombstone.Obj.(*corev1.Endpoints)
		if !ok {
			klog.Errorf("Tombstone contained object that is not a endpoint %v", obj)
			return
		}
	}

	es.addKindAndVersion(endpoint)
	klog.V(4).Infof("delete endpoint: %s", endpoint.Name)

	go syncToNode(watch.Deleted, util.ResourceEndpoint, endpoint)

	syncToStorage(es.ctx, watch.Deleted, util.ResourceEndpoint, endpoint)
}

// getInformer returns informer of this syncer.
func (es *EndpointSyncer) getInformer() cache.SharedIndexInformer {
	return es.Informer
}

func (es *EndpointSyncer) addKindAndVersion(endpoint *corev1.Endpoints) {
	endpoint.APIVersion = "v1"
	endpoint.Kind = "Endpoints"
}
