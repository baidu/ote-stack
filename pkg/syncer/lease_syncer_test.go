package syncer

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	"github.com/baidu/ote-stack/pkg/util"
)

func TestHandleNodeLeaseEvent(t *testing.T) {
	InitSubscriber()
	watchChan := make(chan metav1.WatchEvent)
	EdgeSubscriber.Add(util.ResourceNodeLease, "key", watchChan)

	ctx := newSynceContext(t)
	NewSyncerInformerFactory(ctx)
	syncer := NewLeaseSyncer(ctx)

	lease := &v1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			ResourceVersion: "0",
		},
	}
	key, err := cache.MetaNamespaceKeyFunc(lease)
	if err != nil {
		t.Error(err)
	}

	// test add event
	syncer.handleAddEvent(lease)
	data, err := ctx.Store.LevelDB.Get(util.ResourceNodeLease, key)
	assert.Nil(t, err)

	obj := &v1.Lease{}
	err = json.Unmarshal(data, obj)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "test", obj.Name)

	event := <-EdgeSubscriber.subscriber[util.ResourceNodeLease]["key"]
	assert.Equal(t, string(watch.Added), event.Type)
	assert.Equal(t, lease, event.Object.Object)

	// test update event
	newLease := &v1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			ResourceVersion: "1",
		},
	}

	syncer.handleUpdateEvent(lease, newLease)
	data, err = ctx.Store.LevelDB.Get(util.ResourceNodeLease, key)
	assert.Nil(t, err)

	obj = &v1.Lease{}
	err = json.Unmarshal(data, obj)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "test", obj.Name)
	assert.Equal(t, "1", obj.ResourceVersion)

	event = <-EdgeSubscriber.subscriber[util.ResourceNodeLease]["key"]
	assert.Equal(t, string(watch.Modified), event.Type)
	assert.Equal(t, newLease, event.Object.Object)

	// test delete event
	syncer.handleDeleteEvent(lease)
	data, err = ctx.Store.LevelDB.Get(util.ResourceNodeLease, key)
	assert.NotNil(t, err)

	event = <-EdgeSubscriber.subscriber[util.ResourceNodeLease]["key"]
	assert.Equal(t, string(watch.Deleted), event.Type)
	assert.Equal(t, lease, event.Object.Object)

	EdgeSubscriber.Delete(util.ResourceNodeLease, "key")
	ctx.Store.Close()
	os.RemoveAll("../../db_test/")
}
