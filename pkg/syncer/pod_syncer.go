package syncer

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/util"
)

// PodSyncer is responsible for synchronizing pod from apiserver.
type PodSyncer struct {
	ctx      *SyncContext
	Informer cache.SharedIndexInformer
}

func NewPodSyncer(ctx *SyncContext) Syncer {
	informer, ok := ctx.InformerFactory[DefaultInformerFatory]
	if !ok {
		return nil
	}

	return &PodSyncer{
		ctx:      ctx,
		Informer: informer.Core().V1().Pods().Informer(),
	}
}

func (ps *PodSyncer) startSyncer() error {
	if !ps.ctx.IsValid() {
		return fmt.Errorf("start pod syncer failed: SyncContext is invalid")
	}

	registerInformerHandler(ps)

	return nil
}

// handleAddEvent puts the added pod into persistent storage, and return to edge node as a watch event.
func (ps *PodSyncer) handleAddEvent(obj interface{}) {
	pod := obj.(*corev1.Pod)
	ps.addKindAndVersion(pod)

	klog.V(4).Infof("add pod: %s", pod.Name)

	go syncToNode(watch.Added, util.ResourcePod, pod)

	syncToStorage(ps.ctx, watch.Added, util.ResourcePod, pod)
}

// handleUpdateEvent puts the modified pod into persistent storage, and return to edge node as a watch event.
func (ps *PodSyncer) handleUpdateEvent(old, new interface{}) {
	newPod := new.(*corev1.Pod)
	oldPod := old.(*corev1.Pod)
	if newPod.ResourceVersion == oldPod.ResourceVersion {
		// Periodic resync will send update events for all known Deployments.
		// Two different versions of the same Deployment will always have different RVs.
		return
	}

	ps.addKindAndVersion(newPod)
	klog.V(4).Infof("update pod: %s", newPod.Name)

	go syncToNode(watch.Modified, util.ResourcePod, newPod)

	syncToStorage(ps.ctx, watch.Modified, util.ResourcePod, newPod)
}

// handleDeleteEvent delete the pod from persistent storage, and return to edge node as a watch event.
func (ps *PodSyncer) handleDeleteEvent(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.Errorf("Couldn't get object from tombstone %v", obj)
			return
		}
		pod, ok = tombstone.Obj.(*corev1.Pod)
		if !ok {
			klog.Errorf("Tombstone contained object that is not a pod %v", obj)
			return
		}
	}

	ps.addKindAndVersion(pod)
	klog.V(4).Infof("delete pod: %s", pod.Name)

	go syncToNode(watch.Deleted, util.ResourcePod, pod)

	syncToStorage(ps.ctx, watch.Deleted, util.ResourcePod, pod)
}

// getInformer returns informer of this syncer.
func (ps *PodSyncer) getInformer() cache.SharedIndexInformer {
	return ps.Informer
}

func (ps *PodSyncer) addKindAndVersion(pod *corev1.Pod) {
	pod.APIVersion = "v1"
	pod.Kind = "Pod"
}
