// Package syncer defines methods use to start or stop syncer's work.
package syncer

import (
	"fmt"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/storage"
	"github.com/baidu/ote-stack/pkg/util"
)

var (
	// Informers is the tool for syncer to sync resource from apiserver.
	Syncers map[string]Syncer
	// EdgeSubscriber is the watcher for syncer
	EdgeSubscriber = ResourceSubscriber{}
)

const (
	informerDuration = 10 * time.Second

	// name of syncer's informer factory
	DefaultInformerFatory = "defaultInformer"
	NodeInformerFactory   = "nodeInformer"

	// selector field
	ObjectNameField   = "metadata.name"
	SpecNodeNameField = "spec.nodeName"
)

type EdgeWatcher map[string]chan metav1.WatchEvent
type Subscriber map[string]EdgeWatcher

type ResourceSubscriber struct {
	rwMutex    sync.RWMutex
	subscriber Subscriber
}

func (r *ResourceSubscriber) Add(objType, key string, watchChan chan metav1.WatchEvent) {
	defer r.rwMutex.Unlock()

	r.rwMutex.Lock()
	if _, ok := r.subscriber[objType][key]; !ok {
		r.subscriber[objType][key] = watchChan
	}
}

func (r *ResourceSubscriber) Delete(objType, key string) {
	defer r.rwMutex.Unlock()

	r.rwMutex.Lock()
	if _, ok := r.subscriber[objType][key]; ok {
		delete(r.subscriber[objType], key)
	}
}

func InitSubscriber() {
	subcriber := make(Subscriber)

	// TODO add new syncer's subscriber here
	subcriber[util.ResourcePod] = make(EdgeWatcher)
	subcriber[util.ResourceNode] = make(EdgeWatcher)
	subcriber[util.ResourceNodeLease] = make(EdgeWatcher)
	subcriber[util.ResourceEndpoint] = make(EdgeWatcher)
	subcriber[util.ResourceService] = make(EdgeWatcher)
	subcriber[util.ResourceNamespace] = make(EdgeWatcher)
	subcriber[util.ResourceNetworkPolicy] = make(EdgeWatcher)
	subcriber[util.ResourceConfigMap] = make(EdgeWatcher)
	subcriber[util.ResourceSecret] = make(EdgeWatcher)
	subcriber[util.ResourceCSIDriver] = make(EdgeWatcher)
	subcriber[util.ResourceCSINode] = make(EdgeWatcher)
	subcriber[util.ResourceRuntimeClass] = make(EdgeWatcher)
	subcriber[util.ResourceCSR] = make(EdgeWatcher)

	EdgeSubscriber.subscriber = subcriber
}

func GetSubscriber() *ResourceSubscriber {
	return &EdgeSubscriber
}

// SyncContext defines the context object fot sync client.
type SyncContext struct {
	// NodeName is the edge node's name.
	NodeName string
	// InformerFactory gives access to informers for the sync client.
	InformerFactory map[string]informers.SharedInformerFactory
	// StopChan is the stop channel.
	StopChan chan struct{}
	// KubeClient is the kubernetes client interface for the syncer to use.
	KubeClient kubernetes.Interface
	// Store is the local storage for edgehub to store all the synced resource.
	Store *storage.EdgehubStorage
	// SyncTimeout is used to set timeout for syncers syncing cache from server.
	SyncTimeout int
}

// Syncer is the methods needed to sync resource from master.
type Syncer interface {
	startSyncer() error
	handleAddEvent(obj interface{})
	handleUpdateEvent(old, new interface{})
	handleDeleteEvent(obj interface{})
	getInformer() cache.SharedIndexInformer
}

// registerInformerHandler register methods into syncer's informer.
func registerInformerHandler(s Syncer) {
	s.getInformer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    s.handleAddEvent,
		UpdateFunc: s.handleUpdateEvent,
		DeleteFunc: s.handleDeleteEvent,
	})
}

// registerSyncer adds resource syncer.
func registerSyncer(ctx *SyncContext) {
	Syncers = make(map[string]Syncer)

	// TODO add needed syncer here
	Syncers[util.ResourcePod] = NewPodSyncer(ctx)
	Syncers[util.ResourceNode] = NewNodeSyncer(ctx)
	Syncers[util.ResourceService] = NewServiceSyncer(ctx)
	Syncers[util.ResourceEndpoint] = NewEndpointSyncer(ctx)
	Syncers[util.ResourceConfigMap] = NewConfigMapSyncer(ctx)
	Syncers[util.ResourceSecret] = NewSecretSyncer(ctx)
	Syncers[util.ResourceNamespace] = NewNamespaceSyncer(ctx)
	Syncers[util.ResourceEvent] = NewEventSyncer(ctx)

	if checkResourceExist(ctx, util.ResourceNodeLease) {
		Syncers[util.ResourceNodeLease] = NewLeaseSyncer(ctx)
	}
	if checkResourceExist(ctx, util.ResourceNetworkPolicy) {
		Syncers[util.ResourceNetworkPolicy] = NewNetworkPolicySyncer(ctx)
	}
	if checkResourceExist(ctx, util.ResourceCSIDriver) {
		Syncers[util.ResourceCSIDriver] = NewCSIDriverSyncer(ctx)
	}
	if checkResourceExist(ctx, util.ResourceCSINode) {
		Syncers[util.ResourceCSINode] = NewCSINodeSyncer(ctx)
	}
	if checkResourceExist(ctx, util.ResourceRuntimeClass) {
		Syncers[util.ResourceRuntimeClass] = NewRuntimeClassSyncer(ctx)
	}
	if checkResourceExist(ctx, util.ResourceCSR) {
		Syncers[util.ResourceCSR] = NewCSRSyncer(ctx)
	}
}

func NewSyncerInformerFactory(ctx *SyncContext) {
	ctx.InformerFactory = make(map[string]informers.SharedInformerFactory)

	// TODO add needed informer factory here.
	ctx.InformerFactory[NodeInformerFactory] = informers.NewSharedInformerFactoryWithOptions(ctx.KubeClient, informerDuration,
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector(ObjectNameField, ctx.NodeName).String()
		}))
	ctx.InformerFactory[DefaultInformerFatory] = informers.NewSharedInformerFactory(ctx.KubeClient, informerDuration)
}

func StartSyncer(ctx *SyncContext) error {
	NewSyncerInformerFactory(ctx)
	ctx.StopChan = make(chan struct{})

	if !ctx.IsValid() {
		return fmt.Errorf("start syncer failed, sync ctx is not valid")
	}

	if err := startSyncClient(ctx); err != nil {
		return fmt.Errorf("start syncer failed: %v", err)
	}

	if err := startSyncInformerFatory(ctx); err != nil {
		return fmt.Errorf("start syncer failed: %v", err)
	}

	klog.Infof("start syncer success")
	return nil
}

func StopSyncer(ctx *SyncContext) error {
	if ctx.StopChan == nil || isChanClose(ctx.StopChan) {
		return fmt.Errorf("stop syncer failed: StopChan is invalid")
	}

	close(ctx.StopChan)
	Syncers = nil
	ctx.InformerFactory = nil

	klog.Infof("stop syncer success")
	return nil
}

// IsValid checks if SyncContext is valid.
func (ctx *SyncContext) IsValid() bool {
	if ctx == nil {
		klog.Errorf("SyncContext is nil")
		return false
	}
	if ctx.KubeClient == nil {
		klog.Errorf("KubeClient is nil")
		return false
	}
	if ctx.StopChan == nil {
		klog.Errorf("StopChan is nil")
		return false
	}
	if ctx.InformerFactory == nil {
		klog.Errorf("InformerFactory is nil")
		return false
	}
	if ctx.Store == nil {
		klog.Errorf("Storage is nil")
		return false
	}

	return true
}

// startSyncClient start all the needed informer.
func startSyncClient(ctx *SyncContext) error {
	registerSyncer(ctx)
	for clientName, client := range Syncers {
		if client == nil {
			return fmt.Errorf("start %s sync client failed: syncer is not set", clientName)
		}

		if err := client.startSyncer(); err != nil {
			klog.Errorf("start %s sync client failed: %v", clientName, err)
			return err
		}

		klog.V(2).Infof("start %s sync client success", clientName)
	}
	return nil
}

// startSyncInformerFatory start all the needed informer factory.
func startSyncInformerFatory(ctx *SyncContext) error {
	if ctx.StopChan == nil {
		return fmt.Errorf("start syncer informer factory failed: StopChan is nil")
	}

	for name, factory := range ctx.InformerFactory {
		factory.Start(ctx.StopChan)
		// wait for all caches to sync.
		finishChan := make(chan bool)

		go func() {
			isSyncFinish := true

			syncedMap := factory.WaitForCacheSync(ctx.StopChan)
			for informerType, isSync := range syncedMap {
				if !isSync {
					isSyncFinish = false
					klog.Errorf("informer for %s hasn't synced", informerType.String())
					break
				}
			}
			finishChan <- isSyncFinish
		}()

		select {
		case isSyncerDone := <-finishChan:
			if !isSyncerDone {
				return fmt.Errorf("start syncer informer factory failed: %s hasn't synced", name)
			}
		case <-time.After(time.Second * time.Duration(ctx.SyncTimeout)):
			return fmt.Errorf("start syncer informer factory failed: %s synced timeout", name)
		}

		klog.V(2).Infof("start %s success", name)
	}

	// sync the local storage.
	RefreshStorage(ctx)

	return nil
}

// isChanClose check if channel is close.
func isChanClose(ch chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
	}

	return false
}

// RefreshStorage replace all data in local cache when syncer's informer restarts,
// and syncs the data in cache and LevelDB.
func RefreshStorage(ctx *SyncContext) {
	indexer := make(map[string]cache.Indexer)

	for name, syncer := range Syncers {
		if syncer == nil {
			continue
		}

		indexer[name] = syncer.getInformer().GetIndexer()
	}

	// rebuild cache
	ctx.Store.Cache = indexer
	// refresh LevelDB
	ctx.Store.MergeDelete()
}

// syncToStorage syncs the changed resource to persistent storage.
func syncToStorage(ctx *SyncContext, action watch.EventType, objType string, obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("sync obj to storage failed: %v", err)
		return
	}

	data, err := json.Marshal(obj)
	if err != nil {
		klog.Errorf("sync obj to storage failed: %v", err)
		return
	}

	switch action {
	case watch.Added, watch.Modified:
		ctx.Store.Update(objType, key, data)
	case watch.Deleted:
		ctx.Store.Delete(objType, key)
	default:
		klog.Errorf("unsupported type of action: %s", action)
	}
}

// syncToNode syncs the changed resource to edge node.
func syncToNode(action watch.EventType, objType string, obj runtime.Object) error {
	event := metav1.WatchEvent{
		Type:   string(action),
		Object: runtime.RawExtension{Object: obj},
	}

	EdgeSubscriber.rwMutex.RLock()
	for _, channel := range EdgeSubscriber.subscriber[objType] {
		channel <- event
	}
	EdgeSubscriber.rwMutex.RUnlock()

	return nil
}

// checkResourceExist checks if the resource type is exist in server.
func checkResourceExist(ctx *SyncContext, resource string) bool {
	switch resource {
	case util.ResourceCSIDriver:
		if _, err := ctx.KubeClient.StorageV1beta1().CSIDrivers().List(metav1.ListOptions{}); err != nil && apierrors.IsNotFound(err) {
			return false
		}
	case util.ResourceCSINode:
		if _, err := ctx.KubeClient.StorageV1beta1().CSINodes().List(metav1.ListOptions{}); err != nil && apierrors.IsNotFound(err) {
			return false
		}
	case util.ResourceCSR:
		if _, err := ctx.KubeClient.CertificatesV1beta1().CertificateSigningRequests().List(metav1.ListOptions{}); err != nil && apierrors.IsNotFound(err) {
			return false
		}
	case util.ResourceNodeLease:
		if _, err := ctx.KubeClient.CoordinationV1().Leases("kube-system").List(metav1.ListOptions{}); err != nil && apierrors.IsNotFound(err) {
			return false
		}
	case util.ResourceNetworkPolicy:
		if _, err := ctx.KubeClient.NetworkingV1().NetworkPolicies("kube-system").List(metav1.ListOptions{}); err != nil && apierrors.IsNotFound(err) {
			return false
		}
	case util.ResourceRuntimeClass:
		if _, err := ctx.KubeClient.NodeV1beta1().RuntimeClasses().List(metav1.ListOptions{}); err != nil && apierrors.IsNotFound(err) {
			return false
		}
	default:
		klog.Errorf("unsupported resource type")
		return false
	}

	return true
}
