package backends

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/yunify/metadata-proxy/log"
	"github.com/yunify/metadata-proxy/store"
	"math/rand"
	"testing"
	"time"
)

func init() {
	log.SetLevel("debug")
}

func TestClientSync(t *testing.T) {
	backendNodes := []string{"etcdv3", "etcd"}
	prefix := fmt.Sprintf("/prefix%v", rand.Intn(1000))

	for _, backend := range backendNodes {
		println("Test backend: ", backend)
		stopChan := make(chan bool)
		defer func() {
			stopChan <- true
		}()

		nodes := GetDefaultBackends(backend)

		config := Config{
			Backend:      backend,
			BackendNodes: nodes,
			Prefix:       prefix,
		}
		storeClient, err := New(config)
		assert.Nil(t, err)

		storeClient.Delete("/")
		//assert.Nil(t, err)

		metastore := store.New()
		storeClient.Sync(metastore, stopChan)

		testData := FillTestData(storeClient)
		time.Sleep(1000 * time.Millisecond)
		ValidTestData(t, testData, metastore)

		RandomUpdate(testData, storeClient, 10)
		time.Sleep(1000 * time.Millisecond)
		ValidTestData(t, testData, metastore)

		deletedKey := RandomDelete(testData, storeClient)
		time.Sleep(1000 * time.Millisecond)
		ValidTestData(t, testData, metastore)

		val, ok := metastore.Get(deletedKey)
		assert.False(t, ok)
		assert.Nil(t, val)

		storeClient.Delete("/")
	}
}

func TestSelfMapping(t *testing.T) {

	backendNodes := []string{"etcdv3", "etcd"}
	prefix := fmt.Sprintf("/prefix%v", rand.Intn(1000))

	for _, backend := range backendNodes {
		println("Test backend: ", backend)
		stopChan := make(chan bool)
		defer func() {
			stopChan <- true
		}()
		nodes := GetDefaultBackends(backend)

		config := Config{
			Backend:      backend,
			BackendNodes: nodes,
			Prefix:       prefix,
		}
		storeClient, err := New(config)
		assert.Nil(t, err)

		metastore := store.New()
		storeClient.SyncSelfMapping(metastore, stopChan)

		for i := 0; i < 10; i++ {
			ip := fmt.Sprintf("192.168.1.%v", i)
			mapping := map[string]string{
				"instance": fmt.Sprintf("/instances/%v", i),
			}
			storeClient.RegisterSelfMapping(ip, mapping)
		}
		time.Sleep(1000 * time.Millisecond)
		for i := 0; i < 10; i++ {
			ip := fmt.Sprintf("192.168.1.%v", i)
			val, ok := metastore.Get(ip)
			assert.True(t, ok)
			mapVal, mok := val.(map[string]interface{})
			assert.True(t, mok)
			path := mapVal["instance"]
			assert.Equal(t, path, fmt.Sprintf("/instances/%v", i))
		}
	}
}

func FillTestData(storeClient StoreClient) map[string]string {
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

func RandomUpdate(testData map[string]string, storeClient StoreClient, times int) {
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

func RandomDelete(testData map[string]string, storeClient StoreClient) string {
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
