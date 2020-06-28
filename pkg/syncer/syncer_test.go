package syncer

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/baidu/ote-stack/pkg/storage"
)

func newSynceContext(t *testing.T) *SyncContext {
	store, err := storage.NewEdgehubStore(&storage.Config{
		Path: "../../db_test/",
	})
	if err != nil {
		t.Errorf("%v", err)
		return nil
	}

	return &SyncContext{
		NodeName:    "test",
		KubeClient:  k8sfake.NewSimpleClientset([]runtime.Object{}...),
		Store:       store,
		SyncTimeout: 60,
	}
}

func TestStartAndStopSyncer(t *testing.T) {
	// no store
	ctx := &SyncContext{
		NodeName:    "test",
		KubeClient:  k8sfake.NewSimpleClientset([]runtime.Object{}...),
		SyncTimeout: 60,
	}
	err := StartSyncer(ctx)
	assert.NotNil(t, err)

	// no KubeClient
	ctx = &SyncContext{
		NodeName: "test",
	}
	err = StartSyncer(ctx)
	assert.NotNil(t, err)

	// success start syncer
	ctx = newSynceContext(t)
	err = StartSyncer(ctx)
	assert.NotNil(t, ctx.InformerFactory)
	assert.NotNil(t, ctx.StopChan)
	assert.Nil(t, err)

	// failed to stop syncer
	close(ctx.StopChan)
	err = StopSyncer(ctx)
	assert.NotNil(t, err)

	// success stop syncer
	err = StartSyncer(ctx)
	assert.Nil(t, err)
	err = StopSyncer(ctx)
	assert.Nil(t, err)
	assert.Nil(t, ctx.InformerFactory)
	assert.Nil(t, Syncers)

	ctx.Store.Close()
	os.RemoveAll("../../db_test/")
}
