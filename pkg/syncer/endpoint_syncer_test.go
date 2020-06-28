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

func TestHandleEndpointEvent(t *testing.T) {
	InitSubscriber()
	watchChan := make(chan metav1.WatchEvent)
	EdgeSubscriber.Add(util.ResourceEndpoint, "key", watchChan)

	ctx := newSynceContext(t)
	NewSyncerInformerFactory(ctx)
	syncer := NewEndpointSyncer(ctx)

	ep := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			Namespace:       "ns",
			ResourceVersion: "0",
		},
	}
	key, err := cache.MetaNamespaceKeyFunc(ep)
	if err != nil {
		t.Error(err)
	}

	// test add event
	syncer.handleAddEvent(ep)
	data, err := ctx.Store.LevelDB.Get(util.ResourceEndpoint, key)
	assert.Nil(t, err)

	obj := &corev1.Endpoints{}
	err = json.Unmarshal(data, obj)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "test", obj.Name)
	assert.Equal(t, "ns", obj.Namespace)

	event := <-EdgeSubscriber.subscriber[util.ResourceEndpoint]["key"]
	assert.Equal(t, string(watch.Added), event.Type)
	assert.Equal(t, ep, event.Object.Object)

	// test update event
	newEp := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			Namespace:       "ns",
			ResourceVersion: "1",
		},
	}

	syncer.handleUpdateEvent(ep, newEp)
	data, err = ctx.Store.LevelDB.Get(util.ResourceEndpoint, key)
	assert.Nil(t, err)

	obj = &corev1.Endpoints{}
	err = json.Unmarshal(data, obj)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "test", obj.Name)
	assert.Equal(t, "ns", obj.Namespace)
	assert.Equal(t, "1", obj.ResourceVersion)

	event = <-EdgeSubscriber.subscriber[util.ResourceEndpoint]["key"]
	assert.Equal(t, string(watch.Modified), event.Type)
	assert.Equal(t, newEp, event.Object.Object)

	// test delete event
	syncer.handleDeleteEvent(ep)
	data, err = ctx.Store.LevelDB.Get(util.ResourceEndpoint, key)
	assert.NotNil(t, err)

	event = <-EdgeSubscriber.subscriber[util.ResourceEndpoint]["key"]
	assert.Equal(t, string(watch.Deleted), event.Type)
	assert.Equal(t, ep, event.Object.Object)

	EdgeSubscriber.Delete(util.ResourceEndpoint, "key")
	ctx.Store.Close()
	os.RemoveAll("../../db_test/")
}
