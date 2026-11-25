package CouloyDB

import (
	"errors"
	"testing"
	"time"

	"github.com/Kirov7/CouloyDB/driver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errTaskTTLAppendProbeFailure = errors.New("task ttl append probe failure")

type taskTTLAppendProbeWriter struct {
	driver.IOManager
	failed bool
}

func (w *taskTTLAppendProbeWriter) Write(payload []byte) (int, error) {
	if !w.failed {
		w.failed = true
		return 0, errTaskTTLAppendProbeFailure
	}
	return w.IOManager.Write(payload)
}

func TestTaskTTLAppendFailureKeepsExistingLedger(t *testing.T) {
	options := DefaultOptions()
	options.SyncWrites = false

	db, err := NewCouloyDB(options)
	require.NoError(t, err)
	defer destroyCouloyDB(db)

	key := []byte("append-failure-keeps-ttl")
	require.NoError(t, db.PutWithExpiration(key, []byte("old-value"), time.Hour))

	oldJob := db.ttl.timeHeap.Get(string(key))
	require.NotNil(t, oldJob)

	db.activityFile.Writer = &taskTTLAppendProbeWriter{IOManager: db.activityFile.Writer}

	err = db.Put(key, []byte("new-value"))
	require.ErrorIs(t, err, errTaskTTLAppendProbeFailure)

	job := db.ttl.timeHeap.Get(string(key))
	if assert.NotNil(t, job) {
		assert.Equal(t, oldJob.Expiration, job.Expiration)
	}

	value, err := db.Get(key)
	require.NoError(t, err)
	assert.Equal(t, []byte("old-value"), value)
}

func TestTaskTTLAppendFailureDoesNotAddLedger(t *testing.T) {
	options := DefaultOptions()
	options.SyncWrites = false

	db, err := NewCouloyDB(options)
	require.NoError(t, err)
	defer destroyCouloyDB(db)

	key := []byte("append-failure-new-ttl")
	require.NoError(t, db.setActivityFile())
	db.activityFile.Writer = &taskTTLAppendProbeWriter{IOManager: db.activityFile.Writer}

	err = db.PutWithExpiration(key, []byte("value"), time.Hour)
	require.ErrorIs(t, err, errTaskTTLAppendProbeFailure)

	assert.Nil(t, db.ttl.timeHeap.Get(string(key)))
	assert.Nil(t, db.index.getStrIndex().Get(key))
}
