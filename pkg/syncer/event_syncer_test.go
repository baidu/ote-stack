package syncer

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/tools/cache"

	"github.com/baidu/ote-stack/pkg/util"
)

func TestHandleEvent(t *testing.T) {
	ctx := newSynceContext(t)
	NewSyncerInformerFactory(ctx)
	syncer := NewEventSyncer(ctx)

	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			ResourceVersion: "0",
		},
	}
	key, err := cache.MetaNamespaceKeyFunc(event)
	if err != nil {
		t.Error(err)
	}

	// test add event
	syncer.handleAddEvent(event)
	data, err := ctx.Store.LevelDB.Get(util.ResourceEvent, key)
	assert.Nil(t, err)

	obj := &corev1.Event{}
	err = json.Unmarshal(data, obj)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "test", obj.Name)

	// test update event
	newEvent := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			ResourceVersion: "1",
		},
	}

	syncer.handleUpdateEvent(event, newEvent)
	data, err = ctx.Store.LevelDB.Get(util.ResourceEvent, key)
	assert.Nil(t, err)

	obj = &corev1.Event{}
	err = json.Unmarshal(data, obj)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "test", obj.Name)
	assert.Equal(t, "1", obj.ResourceVersion)

	// test delete event
	syncer.handleDeleteEvent(event)
	data, err = ctx.Store.LevelDB.Get(util.ResourceEvent, key)
	assert.NotNil(t, err)

	ctx.Store.Close()
	os.RemoveAll("../../db_test/")
}
