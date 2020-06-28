// Package storage defines the methods for interacting with local storage
package storage

import (
	"fmt"
	"strings"

	"github.com/syndtr/goleveldb/leveldb"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/baidu/ote-stack/pkg/util"
)

const (
	Speration = "-"
)

type Config struct {
	// Path is levelDB's location.
	Path string
}

type EdgehubStorage struct {
	Cache        map[string]cache.Indexer
	LevelDB      *LevelDB
	RemoteEnable bool
}

func NewEdgehubStore(config *Config) (*EdgehubStorage, error) {
	if config.Path == "" {
		return nil, fmt.Errorf("failed to get EdgehubStore: config is invalid")
	}

	db, err := newEdgehubDB(config.Path)
	if err != nil {
		return nil, err
	}

	indexers := RebuildCache(db)
	return &EdgehubStorage{
		LevelDB: db,
		Cache:   indexers,
	}, nil
}

// Get returns the item specified by key from local storage.
func (s *EdgehubStorage) Get(objType string, key string, isNeedSerialize bool) (interface{}, error) {
	if _, ok := s.Cache[objType]; !ok {
		return nil, fmt.Errorf("get %s-%s obj failed: couldn't find object type indexer", objType, key)
	}

	obj, exist, err := s.Cache[objType].GetByKey(key)
	if err != nil {
		return nil, fmt.Errorf("get %s-%s obj failed: %v", objType, key, err)
	}
	if !exist {
		return nil, fmt.Errorf("get %s-%s obj failed: obj is not found", objType, key)
	}

	if isNeedSerialize {
		data, err := json.Marshal(obj)
		if err != nil {
			return nil, fmt.Errorf("get %s-%s obj failed: %v", objType, key, err)
		}
		return data, nil
	}

	return obj, nil
}

// List returns all the items from local storage.
func (s *EdgehubStorage) List(objType string) []interface{} {
	var ret []interface{}

	if _, ok := s.Cache[objType]; !ok {
		return ret
	}

	return s.Cache[objType].List()
}

// Update sets the data in the local storage to its updated state.
func (s *EdgehubStorage) Update(objType string, key string, data []byte) error {
	if !s.RemoteEnable {
		if _, ok := s.Cache[objType]; !ok {
			klog.Errorf("local storage don't have type %s cache", objType)
		} else {
			obj, err := util.GetObjectFromSerializeData(objType, data)
			if err != nil {
				klog.Errorf("failed to get obj from serialize data: %v", err)
			} else if err := s.Cache[objType].Update(obj); err != nil {
				klog.Errorf("failed to update obj from local cache: %v", err)
			}
		}
	}

	return s.LevelDB.Update(objType, key, data)
}

// Delete removes the item specified by key from local storage.
func (s *EdgehubStorage) Delete(objType string, key string) error {
	if !s.RemoteEnable {
		if _, ok := s.Cache[objType]; !ok {
			klog.Errorf("local storage don't have type %s cache", objType)
		} else {
			obj, exist, err := s.Cache[objType].GetByKey(key)
			if err != nil || !exist {
				klog.Errorf("failed to get obj %s-%s from local cache: %v", objType, key, err)
			} else if err := s.Cache[objType].Delete(obj); err != nil {
				klog.Errorf("failed to delete obj from local cache: %v", err)
			}
		}
	}

	return s.LevelDB.Delete(objType, key)
}

// MergeDelete syncs LevelDB's data according to informer cache.
// Due to syncer's handling, LevelDB always contains the data that in informer cache,
// so it just needs to remove the data which is not in informer cache.
func (s *EdgehubStorage) MergeDelete() error {
	batch := new(leveldb.Batch)

	iter := s.LevelDB.db.NewIterator(nil, nil)
	for iter.Next() {
		name := string(iter.Key())

		objType, key, err := splitKeyName(name)
		if err != nil {
			return fmt.Errorf("merge storage failed: splitKeyName %s error", string(name))
		}

		if _, ok := s.Cache[objType]; !ok {
			batch.Delete(iter.Key())
			continue
		}

		_, exist, err := s.Cache[objType].GetByKey(key)
		if err != nil {
			return fmt.Errorf("merge storage failed: %v", err)
		}
		if !exist {
			batch.Delete(iter.Key())
		}
	}

	s.LevelDB.db.Write(batch, nil)
	iter.Release()

	return nil
}

// Close exit the edgehub's storage.
func (s *EdgehubStorage) Close() {
	s.LevelDB.db.Close()
}

func RebuildCache(db *LevelDB) map[string]cache.Indexer {
	indexer := make(map[string]cache.Indexer)

	iter := db.db.NewIterator(nil, nil)
	for iter.Next() {
		data, err := db.db.Get(iter.Key(), nil)
		if err != nil {
			klog.Errorf("get data from level db err: %v", err)
			continue
		}

		name := string(iter.Key())
		objType, _, err := splitKeyName(name)
		if err != nil {
			klog.Errorf("split key name %s failed: %v", name, err)
			continue
		}

		if _, ok := indexer[objType]; !ok {
			indexer[objType] = cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
		}

		obj, err := util.GetObjectFromSerializeData(objType, data)
		if err != nil {
			klog.Errorf("failed to get obj from serialize data: %v", err)
			continue
		}

		indexer[objType].Add(obj)
	}

	iter.Release()
	return indexer
}

// formKeyName generates the key name of leveldb needed.
func formKeyName(objType, name string) string {
	return objType + Speration + name
}

// splitKeyName splits the key name of leveldb's object to object's type and object's key name.
func splitKeyName(name string) (string, string, error) {
	pos := strings.Index(name, Speration)
	if pos == -1 {
		return "", "", fmt.Errorf("couldn't get key name for: %s", name)
	}

	return name[:pos], name[pos+1:], nil
}
