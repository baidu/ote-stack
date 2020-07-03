package syncer

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/api/node/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	"github.com/baidu/ote-stack/pkg/util"
)

func TestHandleRuntimeClassEvent(t *testing.T) {
	InitSubscriber()
	watchChan := make(chan metav1.WatchEvent)
	EdgeSubscriber.Add(util.ResourceRuntimeClass, "key", watchChan)

	ctx := newSynceContext(t)
	NewSyncerInformerFactory(ctx)
	syncer := NewRuntimeClassSyncer(ctx)

	rc := &v1beta1.RuntimeClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			ResourceVersion: "0",
		},
	}
	key, err := cache.MetaNamespaceKeyFunc(rc)
	if err != nil {
		t.Error(err)
	}

	// test add event
	syncer.handleAddEvent(rc)
	data, err := ctx.Store.LevelDB.Get(util.ResourceRuntimeClass, key)
	assert.Nil(t, err)

	obj := &v1beta1.RuntimeClass{}
	err = json.Unmarshal(data, obj)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "test", obj.Name)

	event := <-EdgeSubscriber.subscriber[util.ResourceRuntimeClass]["key"]
	assert.Equal(t, string(watch.Added), event.Type)
	assert.Equal(t, rc, event.Object.Object)

	// test update event
	newRC := &v1beta1.RuntimeClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			ResourceVersion: "1",
		},
	}

	syncer.handleUpdateEvent(rc, newRC)
	data, err = ctx.Store.LevelDB.Get(util.ResourceRuntimeClass, key)
	assert.Nil(t, err)

	obj = &v1beta1.RuntimeClass{}
	err = json.Unmarshal(data, obj)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "test", obj.Name)
	assert.Equal(t, "1", obj.ResourceVersion)

	event = <-EdgeSubscriber.subscriber[util.ResourceRuntimeClass]["key"]
	assert.Equal(t, string(watch.Modified), event.Type)
	assert.Equal(t, newRC, event.Object.Object)

	// test delete event
	syncer.handleDeleteEvent(rc)
	data, err = ctx.Store.LevelDB.Get(util.ResourceRuntimeClass, key)
	assert.NotNil(t, err)

	event = <-EdgeSubscriber.subscriber[util.ResourceRuntimeClass]["key"]
	assert.Equal(t, string(watch.Deleted), event.Type)
	assert.Equal(t, rc, event.Object.Object)

	ctx.Store.Close()
	os.RemoveAll("../../db_test/")
}
