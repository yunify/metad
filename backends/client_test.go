package backends

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/yunify/metadata-proxy/internal"
	"github.com/yunify/metadata-proxy/store"
	"math/rand"
	"testing"
	"time"
)

func TestStore(t *testing.T) {

	backendNodes := []string{"etcd"}
	prefix := fmt.Sprintf("/prefix%v", rand.Int())

	stopChan := make(chan bool)
	defer func() {
		stopChan <- true
	}()
	for _, backend := range backendNodes {

		nodes := GetDefaultBackends(backend)

		config := Config{
			Backend:      backend,
			BackendNodes: nodes,
		}
		storeClient, err := New(config)
		assert.Nil(t, err)

		storeClient.Delete(prefix)
		//assert.Nil(t, err)

		metastore := store.New()
		storeClient.Sync(prefix, metastore, stopChan)

		testData := internal.FillTestData(prefix, storeClient)
		time.Sleep(1000 * time.Millisecond)
		internal.ValidTestData(t, testData, metastore)

		internal.RandomUpdate(testData, storeClient, 10)
		time.Sleep(1000 * time.Millisecond)
		internal.ValidTestData(t, testData, metastore)

		deletedKey := internal.RandomDelete(testData, storeClient)
		time.Sleep(1000 * time.Millisecond)
		internal.ValidTestData(t, testData, metastore)

		val, ok := metastore.Get(deletedKey)
		assert.False(t, ok)
		assert.Nil(t, val)

		storeClient.Delete(prefix)
	}
}
