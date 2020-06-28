package syncer

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	"github.com/baidu/ote-stack/pkg/util"
)

func TestHandleNamespaceEvent(t *testing.T) {
	InitSubscriber()
	watchChan := make(chan metav1.WatchEvent)
	EdgeSubscriber.Add(util.ResourceNamespace, "key", watchChan)

	ctx := newSynceContext(t)
	NewSyncerInformerFactory(ctx)
	syncer := NewNamespaceSyncer(ctx)

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			ResourceVersion: "0",
		},
	}
	key, err := cache.MetaNamespaceKeyFunc(namespace)
	if err != nil {
		t.Error(err)
	}

	// test add event
	syncer.handleAddEvent(namespace)
	data, err := ctx.Store.LevelDB.Get(util.ResourceNamespace, key)
	assert.Nil(t, err)

	obj := &corev1.Namespace{}
	err = json.Unmarshal(data, obj)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "test", obj.Name)

	event := <-EdgeSubscriber.subscriber[util.ResourceNamespace]["key"]
	assert.Equal(t, string(watch.Added), event.Type)
	assert.Equal(t, namespace, event.Object.Object)

	// test update event
	newNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			ResourceVersion: "1",
		},
	}

	syncer.handleUpdateEvent(namespace, newNamespace)
	data, err = ctx.Store.LevelDB.Get(util.ResourceNamespace, key)
	assert.Nil(t, err)

	obj = &corev1.Namespace{}
	err = json.Unmarshal(data, obj)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "test", obj.Name)
	assert.Equal(t, "1", obj.ResourceVersion)

	event = <-EdgeSubscriber.subscriber[util.ResourceNamespace]["key"]
	assert.Equal(t, string(watch.Modified), event.Type)
	assert.Equal(t, newNamespace, event.Object.Object)

	// test delete event
	syncer.handleDeleteEvent(namespace)
	data, err = ctx.Store.LevelDB.Get(util.ResourceNamespace, key)
	assert.NotNil(t, err)

	event = <-EdgeSubscriber.subscriber[util.ResourceNamespace]["key"]
	assert.Equal(t, string(watch.Deleted), event.Type)
	assert.Equal(t, namespace, event.Object.Object)

	EdgeSubscriber.Delete(util.ResourceNamespace, "key")
	ctx.Store.Close()
	os.RemoveAll("../../db_test/")
}
