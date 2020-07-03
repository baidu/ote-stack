package syncer

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/api/certificates/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	"github.com/baidu/ote-stack/pkg/util"
)

func TestHandleCSREvent(t *testing.T) {
	InitSubscriber()
	watchChan := make(chan metav1.WatchEvent)
	EdgeSubscriber.Add(util.ResourceCSR, "key", watchChan)

	ctx := newSynceContext(t)
	NewSyncerInformerFactory(ctx)
	syncer := NewCSRSyncer(ctx)

	csr := &v1beta1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			Namespace:       "ns",
			ResourceVersion: "0",
		},
	}
	key, err := cache.MetaNamespaceKeyFunc(csr)
	if err != nil {
		t.Error(err)
	}

	// test add event
	syncer.handleAddEvent(csr)
	data, err := ctx.Store.LevelDB.Get(util.ResourceCSR, key)
	assert.Nil(t, err)

	obj := &v1beta1.CertificateSigningRequest{}
	err = json.Unmarshal(data, obj)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "test", obj.Name)
	assert.Equal(t, "ns", obj.Namespace)

	event := <-EdgeSubscriber.subscriber[util.ResourceCSR]["key"]
	assert.Equal(t, string(watch.Added), event.Type)
	assert.Equal(t, csr, event.Object.Object)

	// test update event
	newCSR := &v1beta1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			Namespace:       "ns",
			ResourceVersion: "1",
		},
	}

	syncer.handleUpdateEvent(csr, newCSR)
	data, err = ctx.Store.LevelDB.Get(util.ResourceCSR, key)
	assert.Nil(t, err)

	obj = &v1beta1.CertificateSigningRequest{}
	err = json.Unmarshal(data, obj)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "test", obj.Name)
	assert.Equal(t, "ns", obj.Namespace)
	assert.Equal(t, "1", obj.ResourceVersion)

	event = <-EdgeSubscriber.subscriber[util.ResourceCSR]["key"]
	assert.Equal(t, string(watch.Modified), event.Type)
	assert.Equal(t, newCSR, event.Object.Object)

	// test delete event
	syncer.handleDeleteEvent(csr)
	data, err = ctx.Store.LevelDB.Get(util.ResourceCSR, key)
	assert.NotNil(t, err)

	event = <-EdgeSubscriber.subscriber[util.ResourceCSR]["key"]
	assert.Equal(t, string(watch.Deleted), event.Type)
	assert.Equal(t, csr, event.Object.Object)

	ctx.Store.Close()
	os.RemoveAll("../../db_test/")
}
