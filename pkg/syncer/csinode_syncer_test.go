package syncer

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	"github.com/baidu/ote-stack/pkg/util"
)

func TestHandleCSINodeEvent(t *testing.T) {
	InitSubscriber()
	watchChan := make(chan metav1.WatchEvent)
	EdgeSubscriber.Add(util.ResourceCSINode, "key", watchChan)

	ctx := newSynceContext(t)
	NewSyncerInformerFactory(ctx)
	syncer := NewCSINodeSyncer(ctx)

	cn := &v1.CSINode{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			Namespace:       "ns",
			ResourceVersion: "0",
		},
	}
	key, err := cache.MetaNamespaceKeyFunc(cn)
	if err != nil {
		t.Error(err)
	}

	// test add event
	syncer.handleAddEvent(cn)
	data, err := ctx.Store.LevelDB.Get(util.ResourceCSINode, key)
	assert.Nil(t, err)

	obj := &v1.CSINode{}
	err = json.Unmarshal(data, obj)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "test", obj.Name)
	assert.Equal(t, "ns", obj.Namespace)

	event := <-EdgeSubscriber.subscriber[util.ResourceCSINode]["key"]
	assert.Equal(t, string(watch.Added), event.Type)
	assert.Equal(t, cn, event.Object.Object)

	// test update event
	newCN := &v1.CSINode{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			Namespace:       "ns",
			ResourceVersion: "1",
		},
	}

	syncer.handleUpdateEvent(cn, newCN)
	data, err = ctx.Store.LevelDB.Get(util.ResourceCSINode, key)
	assert.Nil(t, err)

	obj = &v1.CSINode{}
	err = json.Unmarshal(data, obj)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "test", obj.Name)
	assert.Equal(t, "ns", obj.Namespace)
	assert.Equal(t, "1", obj.ResourceVersion)

	event = <-EdgeSubscriber.subscriber[util.ResourceCSINode]["key"]
	assert.Equal(t, string(watch.Modified), event.Type)
	assert.Equal(t, newCN, event.Object.Object)

	// test delete event
	syncer.handleDeleteEvent(cn)
	data, err = ctx.Store.LevelDB.Get(util.ResourceCSINode, key)
	assert.NotNil(t, err)

	event = <-EdgeSubscriber.subscriber[util.ResourceCSINode]["key"]
	assert.Equal(t, string(watch.Deleted), event.Type)
	assert.Equal(t, cn, event.Object.Object)

	EdgeSubscriber.Delete(util.ResourceCSINode, "key")
	ctx.Store.Close()
	os.RemoveAll("../../db_test/")
}
