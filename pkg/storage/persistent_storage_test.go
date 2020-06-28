package storage

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/baidu/ote-stack/pkg/util"
)

func TestLevelDB(t *testing.T) {
	db, err := newEdgehubDB("../../db_test/")
	assert.Nil(t, err)
	assert.NotNil(t, db)

	// get failed
	data, err := db.Get(util.ResourcePod, "test1")
	assert.NotNil(t, err)
	assert.Nil(t, data)

	// update success
	err = db.Update(util.ResourcePod, "test1", []byte("hello"))
	assert.Nil(t, err)

	// get success
	data, err = db.Get(util.ResourcePod, "test1")
	assert.Nil(t, err)
	assert.Equal(t, "hello", string(data))

	// list success
	err = db.Update(util.ResourcePod, "test2", []byte("hi"))
	assert.Nil(t, err)
	list := db.List(util.ResourcePod)
	assert.Equal(t, 2, len(list))
	assert.Equal(t, "hi", string(list[1]))

	// delete success
	err = db.Delete(util.ResourcePod, "test1")
	assert.Nil(t, err)
	err = db.Delete(util.ResourcePod, "test2")
	assert.Nil(t, err)

	db.db.Close()
	os.RemoveAll("../../db_test/")
}
