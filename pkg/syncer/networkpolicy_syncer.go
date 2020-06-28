package syncer

import (
	"fmt"

	"k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/util"
)

// NetworkPolicySyncer is responsible for synchronizing networkpolicy from apiserver.
type NetworkPolicySyncer struct {
	ctx      *SyncContext
	Informer cache.SharedIndexInformer
}

func NewNetworkPolicySyncer(ctx *SyncContext) Syncer {
	informer, ok := ctx.InformerFactory[DefaultInformerFatory]
	if !ok {
		return nil
	}

	return &NetworkPolicySyncer{
		ctx:      ctx,
		Informer: informer.Networking().V1().NetworkPolicies().Informer(),
	}
}

func (ns *NetworkPolicySyncer) startSyncer() error {
	if !ns.ctx.IsValid() {
		return fmt.Errorf("start networkpolicy syncer failed: SyncContext is invalid")
	}

	registerInformerHandler(ns)

	return nil
}

// handleAddEvent puts the added networkpolicy into persistent storage, and return to edge node as a watch event.
func (ns *NetworkPolicySyncer) handleAddEvent(obj interface{}) {
	networkpolicy := obj.(*v1.NetworkPolicy)
	ns.addKindAndVersion(networkpolicy)
	klog.V(4).Infof("add networkpolicy: %s", networkpolicy.Name)

	go syncToNode(watch.Added, util.ResourceNetworkPolicy, networkpolicy)

	syncToStorage(ns.ctx, watch.Added, util.ResourceNetworkPolicy, networkpolicy)
}

// handleUpdateEvent puts the modified networkpolicy into persistent storage, and return to edge node as a watch event.
func (ns *NetworkPolicySyncer) handleUpdateEvent(old, new interface{}) {
	newNetworkPolicy := new.(*v1.NetworkPolicy)
	oldNetworkPolicy := old.(*v1.NetworkPolicy)
	if newNetworkPolicy.ResourceVersion == oldNetworkPolicy.ResourceVersion {
		// Periodic resync will send update events for all known Deployments.
		// Two different versions of the same Deployment will always have different RVs.
		return
	}

	ns.addKindAndVersion(newNetworkPolicy)
	klog.V(4).Infof("update networkpolicy: %s", newNetworkPolicy.Name)

	go syncToNode(watch.Modified, util.ResourceNetworkPolicy, newNetworkPolicy)

	syncToStorage(ns.ctx, watch.Modified, util.ResourceNetworkPolicy, newNetworkPolicy)
}

// handleDeleteEvent delete the networkpolicy from persistent storage, and return to edge node as a watch event.
func (ns *NetworkPolicySyncer) handleDeleteEvent(obj interface{}) {
	networkpolicy, ok := obj.(*v1.NetworkPolicy)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.Errorf("Couldn't get object from tombstone %v", obj)
			return
		}
		networkpolicy, ok = tombstone.Obj.(*v1.NetworkPolicy)
		if !ok {
			klog.Errorf("Tombstone contained object that is not a networkpolicy %v", obj)
			return
		}
	}

	ns.addKindAndVersion(networkpolicy)
	klog.V(4).Infof("delete networkpolicy: %s", networkpolicy.Name)

	go syncToNode(watch.Deleted, util.ResourceNetworkPolicy, networkpolicy)

	syncToStorage(ns.ctx, watch.Deleted, util.ResourceNetworkPolicy, networkpolicy)
}

// getInformer returns informer of this syncer.
func (ns *NetworkPolicySyncer) getInformer() cache.SharedIndexInformer {
	return ns.Informer
}

func (ns *NetworkPolicySyncer) addKindAndVersion(networkpolicy *v1.NetworkPolicy) {
	networkpolicy.APIVersion = "networking.k8s.io/v1"
	networkpolicy.Kind = "NetworkPolicy"
}
