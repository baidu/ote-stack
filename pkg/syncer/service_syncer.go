package syncer

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/util"
)

// ServiceSyncer is responsible for synchronizing service from apiserver.
type ServiceSyncer struct {
	ctx      *SyncContext
	Informer cache.SharedIndexInformer
}

func NewServiceSyncer(ctx *SyncContext) Syncer {
	informer, ok := ctx.InformerFactory[DefaultInformerFatory]
	if !ok {
		return nil
	}

	return &ServiceSyncer{
		ctx:      ctx,
		Informer: informer.Core().V1().Services().Informer(),
	}
}

func (ss *ServiceSyncer) startSyncer() error {
	if !ss.ctx.IsValid() {
		return fmt.Errorf("start service syncer failed: SyncContext is invalid")
	}

	registerInformerHandler(ss)

	return nil
}

// handleAddEvent puts the added service into persistent storage, and return to edge node as a watch event.
func (ss *ServiceSyncer) handleAddEvent(obj interface{}) {
	service := obj.(*corev1.Service)
	ss.addKindAndVersion(service)
	klog.V(4).Infof("add service: %s", service.Name)

	go syncToNode(watch.Added, util.ResourceService, service)

	syncToStorage(ss.ctx, watch.Added, util.ResourceService, service)
}

// handleUpdateEvent puts the modified service into persistent storage, and return to edge node as a watch event.
func (ss *ServiceSyncer) handleUpdateEvent(old, new interface{}) {
	newService := new.(*corev1.Service)
	oldService := old.(*corev1.Service)
	if newService.ResourceVersion == oldService.ResourceVersion {
		// Periodic resync will send update events for all known Deployments.
		// Two different versions of the same Deployment will always have different RVs.
		return
	}

	ss.addKindAndVersion(newService)
	klog.V(4).Infof("update service: %s", newService.Name)

	go syncToNode(watch.Modified, util.ResourceService, newService)

	syncToStorage(ss.ctx, watch.Modified, util.ResourceService, newService)
}

// handleDeleteEvent delete the service from persistent storage, and return to edge node as a watch event.
func (ss *ServiceSyncer) handleDeleteEvent(obj interface{}) {
	service, ok := obj.(*corev1.Service)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.Errorf("Couldn't get object from tombstone %v", obj)
			return
		}
		service, ok = tombstone.Obj.(*corev1.Service)
		if !ok {
			klog.Errorf("Tombstone contained object that is not a service %v", obj)
			return
		}
	}

	ss.addKindAndVersion(service)
	klog.V(4).Infof("delete service: %s", service.Name)

	go syncToNode(watch.Deleted, util.ResourceService, service)

	syncToStorage(ss.ctx, watch.Deleted, util.ResourceService, service)
}

// getInformer returns informer of this syncer.
func (ss *ServiceSyncer) getInformer() cache.SharedIndexInformer {
	return ss.Informer
}

func (ss *ServiceSyncer) addKindAndVersion(service *corev1.Service) {
	service.APIVersion = "v1"
	service.Kind = "Service"
}
