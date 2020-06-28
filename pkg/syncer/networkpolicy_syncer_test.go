package syncer

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	"github.com/baidu/ote-stack/pkg/util"
)

func TestHandleNetworkPolicyEvent(t *testing.T) {
	InitSubscriber()
	watchChan := make(chan metav1.WatchEvent)
	EdgeSubscriber.Add(util.ResourceNetworkPolicy, "key", watchChan)

	ctx := newSynceContext(t)
	NewSyncerInformerFactory(ctx)
	syncer := NewNetworkPolicySyncer(ctx)

	np := &v1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			Namespace:       "ns",
			ResourceVersion: "0",
		},
	}
	key, err := cache.MetaNamespaceKeyFunc(np)
	if err != nil {
		t.Error(err)
	}

	// test add event
	syncer.handleAddEvent(np)
	data, err := ctx.Store.LevelDB.Get(util.ResourceNetworkPolicy, key)
	assert.Nil(t, err)

	obj := &v1.NetworkPolicy{}
	err = json.Unmarshal(data, obj)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "test", obj.Name)
	assert.Equal(t, "ns", obj.Namespace)

	event := <-EdgeSubscriber.subscriber[util.ResourceNetworkPolicy]["key"]
	assert.Equal(t, string(watch.Added), event.Type)
	assert.Equal(t, np, event.Object.Object)

	// test update event
	newNP := &v1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			Namespace:       "ns",
			ResourceVersion: "1",
		},
	}

	syncer.handleUpdateEvent(np, newNP)
	data, err = ctx.Store.LevelDB.Get(util.ResourceNetworkPolicy, key)
	assert.Nil(t, err)

	obj = &v1.NetworkPolicy{}
	err = json.Unmarshal(data, obj)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "test", obj.Name)
	assert.Equal(t, "ns", obj.Namespace)
	assert.Equal(t, "1", obj.ResourceVersion)

	event = <-EdgeSubscriber.subscriber[util.ResourceNetworkPolicy]["key"]
	assert.Equal(t, string(watch.Modified), event.Type)
	assert.Equal(t, newNP, event.Object.Object)

	// test delete event
	syncer.handleDeleteEvent(np)
	data, err = ctx.Store.LevelDB.Get(util.ResourceNetworkPolicy, key)
	assert.NotNil(t, err)

	event = <-EdgeSubscriber.subscriber[util.ResourceNetworkPolicy]["key"]
	assert.Equal(t, string(watch.Deleted), event.Type)
	assert.Equal(t, np, event.Object.Object)

	EdgeSubscriber.Delete(util.ResourceNetworkPolicy, "key")
	ctx.Store.Close()
	os.RemoveAll("../../db_test/")
}
