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

func TestHandlePodEvent(t *testing.T) {
	InitSubscriber()
	watchChan := make(chan metav1.WatchEvent)
	EdgeSubscriber.Add(util.ResourcePod, "key", watchChan)

	ctx := newSynceContext(t)
	NewSyncerInformerFactory(ctx)
	syncer := NewPodSyncer(ctx)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			Namespace:       "ns",
			ResourceVersion: "0",
		},
	}
	key, err := cache.MetaNamespaceKeyFunc(pod)
	if err != nil {
		t.Error(err)
	}

	// test add event
	syncer.handleAddEvent(pod)
	data, err := ctx.Store.LevelDB.Get(util.ResourcePod, key)
	assert.Nil(t, err)

	obj := &corev1.Pod{}
	err = json.Unmarshal(data, obj)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "test", obj.Name)
	assert.Equal(t, "ns", obj.Namespace)

	event := <-EdgeSubscriber.subscriber[util.ResourcePod]["key"]
	assert.Equal(t, string(watch.Added), event.Type)
	assert.Equal(t, pod, event.Object.Object)

	// test update event
	newPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			Namespace:       "ns",
			ResourceVersion: "1",
		},
	}

	syncer.handleUpdateEvent(pod, newPod)
	data, err = ctx.Store.LevelDB.Get(util.ResourcePod, key)
	assert.Nil(t, err)

	obj = &corev1.Pod{}
	err = json.Unmarshal(data, obj)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "test", obj.Name)
	assert.Equal(t, "ns", obj.Namespace)
	assert.Equal(t, "1", obj.ResourceVersion)

	event = <-EdgeSubscriber.subscriber[util.ResourcePod]["key"]
	assert.Equal(t, string(watch.Modified), event.Type)
	assert.Equal(t, newPod, event.Object.Object)

	// test delete event
	syncer.handleDeleteEvent(pod)
	data, err = ctx.Store.LevelDB.Get(util.ResourcePod, key)
	assert.NotNil(t, err)

	event = <-EdgeSubscriber.subscriber[util.ResourcePod]["key"]
	assert.Equal(t, string(watch.Deleted), event.Type)
	assert.Equal(t, pod, event.Object.Object)

	EdgeSubscriber.Delete(util.ResourcePod, "key")
	ctx.Store.Close()
	os.RemoveAll("../../db_test/")
}
