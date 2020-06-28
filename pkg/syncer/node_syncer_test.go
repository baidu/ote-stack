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

func TestHandleNodeEvent(t *testing.T) {
	InitSubscriber()
	watchChan := make(chan metav1.WatchEvent)
	EdgeSubscriber.Add(util.ResourceNode, "key", watchChan)

	ctx := newSynceContext(t)
	NewSyncerInformerFactory(ctx)
	syncer := NewNodeSyncer(ctx)

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			ResourceVersion: "0",
		},
	}
	key, err := cache.MetaNamespaceKeyFunc(node)
	if err != nil {
		t.Error(err)
	}

	// test add event
	syncer.handleAddEvent(node)
	data, err := ctx.Store.LevelDB.Get(util.ResourceNode, key)
	assert.Nil(t, err)

	obj := &corev1.Node{}
	err = json.Unmarshal(data, obj)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "test", obj.Name)

	event := <-EdgeSubscriber.subscriber[util.ResourceNode]["key"]
	assert.Equal(t, string(watch.Added), event.Type)
	assert.Equal(t, node, event.Object.Object)

	// test update event
	newNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			ResourceVersion: "1",
		},
	}

	syncer.handleUpdateEvent(node, newNode)
	data, err = ctx.Store.LevelDB.Get(util.ResourceNode, key)
	assert.Nil(t, err)

	obj = &corev1.Node{}
	err = json.Unmarshal(data, obj)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "test", obj.Name)
	assert.Equal(t, "1", obj.ResourceVersion)

	event = <-EdgeSubscriber.subscriber[util.ResourceNode]["key"]
	assert.Equal(t, string(watch.Modified), event.Type)
	assert.Equal(t, newNode, event.Object.Object)

	// test delete event
	syncer.handleDeleteEvent(node)
	data, err = ctx.Store.LevelDB.Get(util.ResourceNode, key)
	assert.NotNil(t, err)

	event = <-EdgeSubscriber.subscriber[util.ResourceNode]["key"]
	assert.Equal(t, string(watch.Deleted), event.Type)
	assert.Equal(t, node, event.Object.Object)

	EdgeSubscriber.Delete(util.ResourceNode, "key")
	ctx.Store.Close()
	os.RemoveAll("../../db_test/")
}
