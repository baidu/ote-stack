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

func TestHandleServiceEvent(t *testing.T) {
	InitSubscriber()
	watchChan := make(chan metav1.WatchEvent)
	EdgeSubscriber.Add(util.ResourceService, "key", watchChan)

	ctx := newSynceContext(t)
	NewSyncerInformerFactory(ctx)
	syncer := NewServiceSyncer(ctx)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			Namespace:       "ns",
			ResourceVersion: "0",
		},
	}
	key, err := cache.MetaNamespaceKeyFunc(service)
	if err != nil {
		t.Error(err)
	}

	// test add event
	syncer.handleAddEvent(service)
	data, err := ctx.Store.LevelDB.Get(util.ResourceService, key)
	assert.Nil(t, err)

	obj := &corev1.Service{}
	err = json.Unmarshal(data, obj)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "test", obj.Name)
	assert.Equal(t, "ns", obj.Namespace)

	event := <-EdgeSubscriber.subscriber[util.ResourceService]["key"]
	assert.Equal(t, string(watch.Added), event.Type)
	assert.Equal(t, service, event.Object.Object)

	// test update event
	newService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			Namespace:       "ns",
			ResourceVersion: "1",
		},
	}

	syncer.handleUpdateEvent(service, newService)
	data, err = ctx.Store.LevelDB.Get(util.ResourceService, key)
	assert.Nil(t, err)

	obj = &corev1.Service{}
	err = json.Unmarshal(data, obj)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "test", obj.Name)
	assert.Equal(t, "ns", obj.Namespace)
	assert.Equal(t, "1", obj.ResourceVersion)

	event = <-EdgeSubscriber.subscriber[util.ResourceService]["key"]
	assert.Equal(t, string(watch.Modified), event.Type)
	assert.Equal(t, newService, event.Object.Object)

	// test delete event
	syncer.handleDeleteEvent(service)
	data, err = ctx.Store.LevelDB.Get(util.ResourceService, key)
	assert.NotNil(t, err)

	event = <-EdgeSubscriber.subscriber[util.ResourceService]["key"]
	assert.Equal(t, string(watch.Deleted), event.Type)
	assert.Equal(t, service, event.Object.Object)

	EdgeSubscriber.Delete(util.ResourceService, "key")
	ctx.Store.Close()
	os.RemoveAll("../../db_test/")
}
