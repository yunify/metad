package metadata

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/yunify/metadata-proxy/backends"
	"github.com/yunify/metadata-proxy/log"
	"github.com/yunify/metadata-proxy/store"
	"math/rand"
	"testing"
	"time"
)

func TestMetarepo(t *testing.T) {

	backendNodes := []string{"etcd"}
	prefix := fmt.Sprintf("/prefix%v", rand.Intn(1000))

	for _, backend := range backendNodes {

		nodes := backends.GetDefaultBackends(backend)

		config := backends.Config{
			Backend:      backend,
			BackendNodes: nodes,
			Prefix:       prefix,
		}
		storeClient, err := backends.New(config)
		assert.Nil(t, err)

		storeClient.Delete("/")
		//assert.Nil(t, err)

		metastore := store.New()

		selfMapping := make(map[string]map[string]string)
		metarepo := New(prefix, selfMapping, storeClient, metastore)
		metarepo.StartSync()

		testData := FillTestData(storeClient)
		time.Sleep(1000 * time.Millisecond)
		ValidTestData(t, testData, metastore)

		val, ok := metarepo.Get("/0")
		assert.True(t, ok)
		assert.NotNil(t, val)

		mapVal, mok := val.(map[string]interface{})
		assert.True(t, mok)

		_, mok = mapVal["0"]
		assert.True(t, mok)

		storeClient.Delete("/0/0")

		//TODO etcd current not support watch children delete. so try resync

		metarepo.ReSync()
		time.Sleep(1000 * time.Millisecond)

		_, ok = metarepo.Get("/0/0")
		assert.False(t, ok)

		storeClient.Delete("/")
	}
}

func FillTestData(storeClient backends.StoreClient) map[string]string {
	testData := make(map[string]string)
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			key := fmt.Sprintf("/%v/%v", i, j)
			val := fmt.Sprintf("%v-%v", i, j)
			testData[key] = val
		}
	}
	err := storeClient.SetValues(testData)
	if err != nil {
		log.Error("SetValues error", err.Error())
	}
	return testData
}

func RandomUpdate(testData map[string]string, storeClient backends.StoreClient, times int) {
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

func RandomDelete(testData map[string]string, storeClient backends.StoreClient) string {
	length := len(testData)
	keys := make([]string, 0, length)
	for k := range testData {
		keys = append(keys, k)
	}
	idx := rand.Intn(length)
	key := keys[idx]
	storeClient.Delete(key)
	delete(testData, key)
	return key
}

func ValidTestData(t *testing.T, testData map[string]string, metastore store.Store) {
	for k, v := range testData {
		storeVal, _ := metastore.Get(k)
		assert.Equal(t, v, storeVal)
	}
}
