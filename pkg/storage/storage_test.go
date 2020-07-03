package storage

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

func newTestPod(name, namespace, version string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			ResourceVersion: version,
		},
	}
}

func TestStorage(t *testing.T) {
	config := &Config{
		Path: "../../db_test/",
	}

	storage, err := NewEdgehubStore(config)
	if err != nil {
		t.Error(err)
	}
	assert.NotNil(t, storage)

	indexers := map[string]cache.Indexer{}
	indexers[util.ResourcePod] = cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	storage.Cache = indexers

	// test Get action
	storage.RemoteEnable = true

	pod := newTestPod("test", "ns", "0")
	storage.Cache[util.ResourcePod].Add(pod)

	key, err := cache.MetaNamespaceKeyFunc(pod)
	if err != nil {
		t.Error(err)
	}
	data, err := storage.Get(util.ResourcePod, key, true)
	assert.Nil(t, err)

	obj := &corev1.Pod{}
	err = json.Unmarshal(data.([]byte), obj)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "test", obj.Name)
	assert.Equal(t, "ns", obj.Namespace)

	// test Update action
	storage.RemoteEnable = false

	newPod := newTestPod("test", "ns", "1")
	data, err = json.Marshal(newPod)
	if err != nil {
		t.Error(err)
	}

	key, err = cache.MetaNamespaceKeyFunc(pod)
	if err != nil {
		t.Error(err)
	}

	err = storage.Update(util.ResourcePod, key, data.([]byte))
	assert.Nil(t, err)

	data, err = storage.Get(util.ResourcePod, key, true)
	assert.Nil(t, err)
	obj = &corev1.Pod{}
	err = json.Unmarshal(data.([]byte), obj)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "test", obj.Name)
	assert.Equal(t, "ns", obj.Namespace)
	assert.Equal(t, "1", obj.ResourceVersion)

	// test List action
	list1 := storage.List(util.ResourcePod)
	assert.Equal(t, 1, len(list1))

	list1 = storage.List(util.ResourceService)
	assert.Equal(t, 0, len(list1))

	storage.RemoteEnable = true
	list2 := storage.List(util.ResourcePod)
	assert.Equal(t, 1, len(list2))

	list2 = storage.List(util.ResourceService)
	assert.Equal(t, 0, len(list2))

	// test delete action
	err = storage.Delete(util.ResourcePod, key)
	assert.Nil(t, err)

	storage.Delete(util.ResourceService, key)
	assert.Nil(t, err)

	storage.Close()
	os.RemoveAll("../../db_test/")
}

func TestEdgehubStorage_MergeDelete(t *testing.T) {
	config := &Config{
		Path: "../../db_test/",
	}

	storage, err := NewEdgehubStore(config)
	if err != nil {
		t.Error(err)
	}
	assert.NotNil(t, storage)

	storage.RemoteEnable = true
	indexers := map[string]cache.Indexer{}
	indexers[util.ResourcePod] = cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	storage.Cache = indexers

	pod1 := newTestPod("test1", "ns", "0")
	data1, err := json.Marshal(pod1)
	if err != nil {
		t.Error(err)
	}

	key1, err := cache.MetaNamespaceKeyFunc(pod1)
	if err != nil {
		t.Error(err)
	}
	err = storage.Update(util.ResourcePod, key1, data1)
	assert.Nil(t, err)
	storage.Cache[util.ResourcePod].Add(pod1)

	pod2 := newTestPod("test2", "ns", "0")
	data2, err := json.Marshal(pod2)
	if err != nil {
		t.Error(err)
	}

	key2, err := cache.MetaNamespaceKeyFunc(pod2)
	if err != nil {
		t.Error(err)
	}
	err = storage.Update(util.ResourcePod, key2, data2)
	assert.Nil(t, err)

	// success merge
	err = storage.MergeDelete()
	assert.Nil(t, err)

	list := storage.LevelDB.List(util.ResourcePod)
	assert.Equal(t, 1, len(list))
	assert.Equal(t, data1, list[0])

	// clear storage's items when using to test
	err = storage.Delete(util.ResourcePod, key1)
	assert.Nil(t, err)
	err = storage.Delete(util.ResourcePod, key2)
	assert.Nil(t, err)

	storage.Close()
	os.RemoveAll("../../db_test/")
}

func TestRebuildCache(t *testing.T) {
	levelDB, err := newEdgehubDB("../../db_test/")
	assert.Nil(t, err)
	assert.NotNil(t, levelDB)

	indexers := RebuildCache(levelDB)
	assert.Equal(t, 0, len(indexers))

	pod := newTestPod("test", "ns", "0")
	data, err := json.Marshal(pod)
	if err != nil {
		t.Error(err)
	}

	key, err := cache.MetaNamespaceKeyFunc(pod)
	if err != nil {
		t.Error(err)
	}

	err = levelDB.Update(util.ResourcePod, key, data)
	assert.Nil(t, err)

	indexers = RebuildCache(levelDB)
	assert.Equal(t, 1, len(indexers))

	err = levelDB.Delete(util.ResourcePod, key)
	assert.Nil(t, err)

	levelDB.db.Close()
	os.RemoveAll("../../db_test/")
}
