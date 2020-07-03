package syncer

import (
	"fmt"

	"k8s.io/api/certificates/v1beta1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/util"
)

// CSRSyncer is responsible for synchronizing csr from apiserver.
type CSRSyncer struct {
	ctx      *SyncContext
	Informer cache.SharedIndexInformer
}

func NewCSRSyncer(ctx *SyncContext) Syncer {
	informer, ok := ctx.InformerFactory[DefaultInformerFatory]
	if !ok {
		return nil
	}

	return &CSRSyncer{
		ctx:      ctx,
		Informer: informer.Certificates().V1beta1().CertificateSigningRequests().Informer(),
	}
}

func (cs *CSRSyncer) startSyncer() error {
	if !cs.ctx.IsValid() {
		return fmt.Errorf("start csr syncer failed: SyncContext is invalid")
	}

	registerInformerHandler(cs)

	return nil
}

// handleAddEvent puts the added csr into persistent storage, and return to edge node as a watch event.
func (cs *CSRSyncer) handleAddEvent(obj interface{}) {
	csr := obj.(*v1beta1.CertificateSigningRequest)
	cs.addKindAndVersion(csr)
	klog.V(4).Infof("add csr: %s", csr.Name)

	go syncToNode(watch.Added, util.ResourceCSR, csr)

	syncToStorage(cs.ctx, watch.Added, util.ResourceCSR, csr)
}

// handleUpdateEvent puts the modified csr into persistent storage, and return to edge node as a watch event.
func (cs *CSRSyncer) handleUpdateEvent(old, new interface{}) {
	newCSR := new.(*v1beta1.CertificateSigningRequest)
	oldCSR := old.(*v1beta1.CertificateSigningRequest)
	if newCSR.ResourceVersion == oldCSR.ResourceVersion {
		// Periodic resync will send update events for all known Deployments.
		// Two different versions of the same Deployment will always have different RVs.
		return
	}

	cs.addKindAndVersion(newCSR)
	klog.V(4).Infof("update csr: %s", newCSR.Name)

	go syncToNode(watch.Modified, util.ResourceCSR, newCSR)

	syncToStorage(cs.ctx, watch.Modified, util.ResourceCSR, newCSR)
}

// handleDeleteEvent delete the csr from persistent storage, and return to edge node as a watch event.
func (cs *CSRSyncer) handleDeleteEvent(obj interface{}) {
	csr, ok := obj.(*v1beta1.CertificateSigningRequest)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.Errorf("Couldn't get object from tombstone %v", obj)
			return
		}
		csr, ok = tombstone.Obj.(*v1beta1.CertificateSigningRequest)
		if !ok {
			klog.Errorf("Tombstone contained object that is not a csr %v", obj)
			return
		}
	}

	cs.addKindAndVersion(csr)
	klog.V(4).Infof("delete csr: %s", csr.Name)

	go syncToNode(watch.Deleted, util.ResourceCSR, csr)

	syncToStorage(cs.ctx, watch.Deleted, util.ResourceCSR, csr)
}

// getInformer returns informer of this syncer.
func (cs *CSRSyncer) getInformer() cache.SharedIndexInformer {
	return cs.Informer
}

func (cs *CSRSyncer) addKindAndVersion(csr *v1beta1.CertificateSigningRequest) {
	csr.APIVersion = "certificates.k8s.io/v1beta1"
	csr.Kind = "CertificateSigningRequest"
}
