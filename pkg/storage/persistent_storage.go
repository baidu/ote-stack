package storage

import (
	"fmt"

	"k8s.io/klog"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

type LevelDB struct {
	db *leveldb.DB
}

func newEdgehubDB(path string) (*LevelDB, error) {
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open leveldb: %v", err)
	}

	return &LevelDB{db: db}, nil
}

func (l *LevelDB) Get(objType string, key string) ([]byte, error) {
	data, err := l.db.Get([]byte(formKeyName(objType, key)), nil)
	if err != nil {
		return nil, fmt.Errorf("get %s-%s obj failed: %v", objType, key, err)
	}

	return data, nil
}

func (l *LevelDB) List(objType string) [][]byte {
	var list [][]byte
	iter := l.db.NewIterator(util.BytesPrefix([]byte(objType+Speration)), nil)

	for iter.Next() {
		data, err := l.db.Get(iter.Key(), nil)
		if err != nil {
			klog.Infof("level db err: %v", err)
			continue
		}
		list = append(list, data)
	}

	iter.Release()
	return list
}

func (l *LevelDB) Update(objType string, key string, data []byte) error {
	if err := l.db.Put([]byte(formKeyName(objType, key)), data, nil); err != nil {
		return fmt.Errorf("update %s-%s obj failed: %v", objType, key, err)
	}

	return nil
}

func (l *LevelDB) Delete(objType string, key string) error {
	if err := l.db.Delete([]byte(formKeyName(objType, key)), nil); err != nil {
		return fmt.Errorf("delete %s-%s obj failed: %v", objType, key, err)
	}

	return nil
}
