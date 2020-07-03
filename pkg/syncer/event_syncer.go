package syncer

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/util"
)

// EventSyncer is responsible for synchronizing event from apiserver.
type EventSyncer struct {
	ctx      *SyncContext
	Informer cache.SharedIndexInformer
}

func NewEventSyncer(ctx *SyncContext) Syncer {
	informer, ok := ctx.InformerFactory[DefaultInformerFatory]
	if !ok {
		return nil
	}

	return &EventSyncer{
		ctx:      ctx,
		Informer: informer.Core().V1().Events().Informer(),
	}
}

func (es *EventSyncer) startSyncer() error {
	if !es.ctx.IsValid() {
		return fmt.Errorf("start event syncer failed: SyncContext is invalid")
	}

	registerInformerHandler(es)

	return nil
}

// handleAddNode puts the added event into persistent storage, and return to edge node as a watch event.
func (es *EventSyncer) handleAddEvent(obj interface{}) {
	event := obj.(*corev1.Event)
	es.addKindAndVersion(event)

	go syncToNode(watch.Added, util.ResourceEvent, event)

	syncToStorage(es.ctx, watch.Added, util.ResourceEvent, event)
}

// handleUpdateEvent puts the modified event into persistent storage, and return to edge node as a watch event.
func (es *EventSyncer) handleUpdateEvent(old, new interface{}) {
	newEvent := new.(*corev1.Event)
	oldEvent := old.(*corev1.Event)
	if newEvent.ResourceVersion == oldEvent.ResourceVersion {
		// Periodic resync will send update events for all known Deployments.
		// Two different versions of the same Deployment will always have different RVs.
		return
	}

	es.addKindAndVersion(newEvent)

	go syncToNode(watch.Modified, util.ResourceEvent, newEvent)

	syncToStorage(es.ctx, watch.Modified, util.ResourceEvent, newEvent)
}

// handleDeleteEvent delete the event from persistent storage, and return to edge node as a watch event.
func (es *EventSyncer) handleDeleteEvent(obj interface{}) {
	event, ok := obj.(*corev1.Event)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.Errorf("Couldn't get object from tombstone %v", obj)
			return
		}
		event, ok = tombstone.Obj.(*corev1.Event)
		if !ok {
			klog.Errorf("Tombstone contained object that is not a event %v", obj)
			return
		}
	}

	es.addKindAndVersion(event)

	go syncToNode(watch.Deleted, util.ResourceEvent, event)

	syncToStorage(es.ctx, watch.Deleted, util.ResourceEvent, event)
}

// getInformer returns informer of this syncer.
func (es *EventSyncer) getInformer() cache.SharedIndexInformer {
	return es.Informer
}

func (es *EventSyncer) addKindAndVersion(event *corev1.Event) {
	event.APIVersion = "v1"
	event.Kind = "Node"
}
