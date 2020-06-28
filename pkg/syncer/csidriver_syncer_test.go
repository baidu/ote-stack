package syncer

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/api/storage/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	"github.com/baidu/ote-stack/pkg/util"
)

func TestHandleCSIDriverEvent(t *testing.T) {
	InitSubscriber()
	watchChan := make(chan metav1.WatchEvent)
	EdgeSubscriber.Add(util.ResourceCSIDriver, "key", watchChan)

	ctx := newSynceContext(t)
	NewSyncerInformerFactory(ctx)
	syncer := NewCSIDriverSyncer(ctx)

	cd := &v1beta1.CSIDriver{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			Namespace:       "ns",
			ResourceVersion: "0",
		},
	}
	key, err := cache.MetaNamespaceKeyFunc(cd)
	if err != nil {
		t.Error(err)
	}

	// test add event
	syncer.handleAddEvent(cd)
	data, err := ctx.Store.LevelDB.Get(util.ResourceCSIDriver, key)
	assert.Nil(t, err)

	obj := &v1beta1.CSIDriver{}
	err = json.Unmarshal(data, obj)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "test", obj.Name)
	assert.Equal(t, "ns", obj.Namespace)

	event := <-EdgeSubscriber.subscriber[util.ResourceCSIDriver]["key"]
	assert.Equal(t, string(watch.Added), event.Type)
	assert.Equal(t, cd, event.Object.Object)

	// test update event
	newCD := &v1beta1.CSIDriver{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			Namespace:       "ns",
			ResourceVersion: "1",
		},
	}

	syncer.handleUpdateEvent(cd, newCD)
	data, err = ctx.Store.LevelDB.Get(util.ResourceCSIDriver, key)
	assert.Nil(t, err)

	obj = &v1beta1.CSIDriver{}
	err = json.Unmarshal(data, obj)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "test", obj.Name)
	assert.Equal(t, "ns", obj.Namespace)
	assert.Equal(t, "1", obj.ResourceVersion)

	event = <-EdgeSubscriber.subscriber[util.ResourceCSIDriver]["key"]
	assert.Equal(t, string(watch.Modified), event.Type)
	assert.Equal(t, newCD, event.Object.Object)

	// test delete event
	syncer.handleDeleteEvent(cd)
	data, err = ctx.Store.LevelDB.Get(util.ResourceCSIDriver, key)
	assert.NotNil(t, err)

	event = <-EdgeSubscriber.subscriber[util.ResourceCSIDriver]["key"]
	assert.Equal(t, string(watch.Deleted), event.Type)
	assert.Equal(t, cd, event.Object.Object)

	EdgeSubscriber.Delete(util.ResourceCSIDriver, "key")
	ctx.Store.Close()
	os.RemoveAll("../../db_test/")
}
