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

func TestHandleSecretEvent(t *testing.T) {
	InitSubscriber()
	watchChan := make(chan metav1.WatchEvent)
	EdgeSubscriber.Add(util.ResourceSecret, "key", watchChan)

	ctx := newSynceContext(t)
	NewSyncerInformerFactory(ctx)
	syncer := NewSecretSyncer(ctx)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			Namespace:       "ns",
			ResourceVersion: "0",
		},
	}
	key, err := cache.MetaNamespaceKeyFunc(secret)
	if err != nil {
		t.Error(err)
	}

	// test add event
	syncer.handleAddEvent(secret)
	data, err := ctx.Store.LevelDB.Get(util.ResourceSecret, key)
	assert.Nil(t, err)

	obj := &corev1.Secret{}
	err = json.Unmarshal(data, obj)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "test", obj.Name)
	assert.Equal(t, "ns", obj.Namespace)

	event := <-EdgeSubscriber.subscriber[util.ResourceSecret]["key"]
	assert.Equal(t, string(watch.Added), event.Type)
	assert.Equal(t, secret, event.Object.Object)

	// test update event
	newSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			Namespace:       "ns",
			ResourceVersion: "1",
		},
	}

	syncer.handleUpdateEvent(secret, newSecret)
	data, err = ctx.Store.LevelDB.Get(util.ResourceSecret, key)
	assert.Nil(t, err)

	obj = &corev1.Secret{}
	err = json.Unmarshal(data, obj)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "test", obj.Name)
	assert.Equal(t, "ns", obj.Namespace)
	assert.Equal(t, "1", obj.ResourceVersion)

	event = <-EdgeSubscriber.subscriber[util.ResourceSecret]["key"]
	assert.Equal(t, string(watch.Modified), event.Type)
	assert.Equal(t, newSecret, event.Object.Object)

	// test delete event
	syncer.handleDeleteEvent(secret)
	data, err = ctx.Store.LevelDB.Get(util.ResourceSecret, key)
	assert.NotNil(t, err)

	event = <-EdgeSubscriber.subscriber[util.ResourceSecret]["key"]
	assert.Equal(t, string(watch.Deleted), event.Type)
	assert.Equal(t, secret, event.Object.Object)

	EdgeSubscriber.Delete(util.ResourceSecret, "key")
	ctx.Store.Close()
	os.RemoveAll("../../db_test/")
}
