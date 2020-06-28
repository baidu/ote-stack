package syncer

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/util"
)

// SecretSyncer is responsible for synchronizing secret from apiserver.
type SecretSyncer struct {
	ctx      *SyncContext
	Informer cache.SharedIndexInformer
}

func NewSecretSyncer(ctx *SyncContext) Syncer {
	informer, ok := ctx.InformerFactory[DefaultInformerFatory]
	if !ok {
		return nil
	}

	return &SecretSyncer{
		ctx:      ctx,
		Informer: informer.Core().V1().Secrets().Informer(),
	}
}

func (ss *SecretSyncer) startSyncer() error {
	if !ss.ctx.IsValid() {
		return fmt.Errorf("start secret syncer failed: SyncContext is invalid")
	}

	registerInformerHandler(ss)

	return nil
}

// handleAddEvent puts the added secret into persistent storage, and return to edge node as a watch event.
func (ss *SecretSyncer) handleAddEvent(obj interface{}) {
	secret := obj.(*corev1.Secret)
	ss.addKindAndVersion(secret)
	klog.V(4).Infof("add secret: %s", secret.Name)

	go syncToNode(watch.Added, util.ResourceSecret, secret)

	syncToStorage(ss.ctx, watch.Added, util.ResourceSecret, secret)
}

// handleUpdateEvent puts the modified secret into persistent storage, and return to edge node as a watch event.
func (ss *SecretSyncer) handleUpdateEvent(old, new interface{}) {
	newSecret := new.(*corev1.Secret)
	oldSecret := old.(*corev1.Secret)
	if newSecret.ResourceVersion == oldSecret.ResourceVersion {
		// Periodic resync will send update events for all known Deployments.
		// Two different versions of the same Deployment will always have different RVs.
		return
	}

	ss.addKindAndVersion(newSecret)
	klog.V(4).Infof("update secret: %s", newSecret.Name)

	go syncToNode(watch.Modified, util.ResourceSecret, newSecret)

	syncToStorage(ss.ctx, watch.Modified, util.ResourceSecret, newSecret)
}

// handleDeleteEvent delete the secret from persistent storage, and return to edge node as a watch event.
func (ss *SecretSyncer) handleDeleteEvent(obj interface{}) {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.Errorf("Couldn't get object from tombstone %v", obj)
			return
		}
		secret, ok = tombstone.Obj.(*corev1.Secret)
		if !ok {
			klog.Errorf("Tombstone contained object that is not a secret %v", obj)
			return
		}
	}

	ss.addKindAndVersion(secret)
	klog.V(4).Infof("delete secret: %s", secret.Name)

	go syncToNode(watch.Deleted, util.ResourceSecret, secret)

	syncToStorage(ss.ctx, watch.Deleted, util.ResourceSecret, secret)
}

// getInformer returns informer of this syncer.
func (ss *SecretSyncer) getInformer() cache.SharedIndexInformer {
	return ss.Informer
}

func (ss *SecretSyncer) addKindAndVersion(secret *corev1.Secret) {
	secret.APIVersion = "v1"
	secret.Kind = "Secret"
}
