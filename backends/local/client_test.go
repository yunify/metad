package local

import (
	"github.com/stretchr/testify/assert"
	"github.com/yunify/metad/log"
	"github.com/yunify/metad/store"
	"math/rand"
	"testing"
	"time"
)

func init() {
	log.SetLevel("debug")
	rand.Seed(int64(time.Now().Nanosecond()))
}

func TestClientSyncStop(t *testing.T) {

	stopChan := make(chan bool)

	storeClient, err := NewLocalClient()
	assert.NoError(t, err)

	go func() {
		time.Sleep(3000 * time.Millisecond)
		stopChan <- true
	}()

	metastore := store.New()
	// expect internalSync not block after stopChan has signal
	storeClient.internalSync("data", storeClient.data, metastore, stopChan)
}
