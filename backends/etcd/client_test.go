package etcd

import (
	"fmt"
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

	prefix := fmt.Sprintf("/prefix%v", rand.Intn(1000))

	stopChan := make(chan bool)

	nodes := []string{"http://127.0.0.1:2379"}
	storeClient, err := NewEtcdClient("default", prefix, nodes, "", "", "", false, "", "")
	assert.NoError(t, err)

	time.Sleep(3000 * time.Millisecond)
	go func() {
		stopChan <- true
	}()

	metastore := store.New()
	// expect internalSync not block after stopChan has signal
	storeClient.internalSync(prefix, metastore, stopChan)
}
