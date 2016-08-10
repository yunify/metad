package backends

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/yunify/metadata-proxy/store"
	"math/rand"
	"testing"
	"time"
)

func TestStore(t *testing.T) {

	backends := []string{"etcd"}
	prefix := fmt.Sprintf("/prefix%v", rand.Int())

	stopChan := make(chan bool)
	for _, backend := range backends {

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

		testData := fillTestData(prefix, storeClient)
		time.Sleep(1000 * time.Millisecond)
		validTestData(t, testData, metastore)

		randomUpdate(testData, storeClient, 10)
		time.Sleep(1000 * time.Millisecond)
		validTestData(t, testData, metastore)

		randomDelete(testData, storeClient)
		time.Sleep(1000 * time.Millisecond)
		validTestData(t, testData, metastore)

		storeClient.Delete(prefix)
	}
}

func fillTestData(prefix string, storeClient StoreClient) map[string]string {
	testData := make(map[string]string)
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			key := fmt.Sprintf("%s/%v/%v", prefix, i, j)
			val := fmt.Sprintf("%v-%v", i, j)
			testData[key] = val
		}
	}
	storeClient.SetValues(testData)
	return testData
}

func randomUpdate(testData map[string]string, storeClient StoreClient, times int) {
	length := len(testData)
	keys := make([]string, 0, length)
	for k := range testData {
		keys = append(keys, k)
	}
	for i := 0; i < times; i++ {
		idx := rand.Intn(length)
		key := keys[idx]
		val := testData[key]
		newVal := fmt.Sprintf("%s-%v", val, 0)

		storeClient.SetValues(map[string]string{key: newVal})
		testData[key] = newVal
	}
}

func randomDelete(testData map[string]string, storeClient StoreClient) {
	length := len(testData)
	keys := make([]string, 0, length)
	for k := range testData {
		keys = append(keys, k)
	}
	idx := rand.Intn(length)
	key := keys[idx]
	storeClient.Delete(key)
	delete(testData, key)
}

func validTestData(t *testing.T, testData map[string]string, metastore store.Store) {
	for k, v := range testData {
		storeVal, _ := metastore.Get(k)
		assert.Equal(t, v, storeVal)
	}
}
