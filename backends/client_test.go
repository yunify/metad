package backends

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/yunify/metadata-proxy/log"
	"github.com/yunify/metadata-proxy/store"
	"github.com/yunify/metadata-proxy/util/flatmap"
	"math/rand"
	"testing"
	"time"
)

var (
	backendNodes = []string{
		"etcd",
		"etcdv3",
	}
)

func init() {
	log.SetLevel("debug")
	rand.Seed(int64(time.Now().Nanosecond()))
}

func TestClientGetPut(t *testing.T) {
	for _, backend := range backendNodes {
		println("Test backend: ", backend)

		prefix := fmt.Sprintf("/prefix%v", rand.Intn(1000))

		nodes := GetDefaultBackends(backend)

		config := Config{
			Backend:      backend,
			BackendNodes: nodes,
			Prefix:       prefix,
		}
		storeClient, err := New(config)
		assert.NoError(t, err)

		storeClient.Delete("/", true)

		err = storeClient.Put("testkey", "testvalue", false)
		assert.NoError(t, err)

		val, getErr := storeClient.Get("testkey", false)
		assert.NoError(t, getErr)
		assert.Equal(t, "testvalue", val)

		// test no exist key
		val, getErr = storeClient.Get("noexistkey", false)
		assert.NoError(t, getErr)
		assert.Equal(t, "", val)

		storeClient.Delete("/", true)
	}
}

func TestClientGetsPuts(t *testing.T) {
	for _, backend := range backendNodes {
		println("Test backend: ", backend)

		prefix := fmt.Sprintf("/prefix%v", rand.Intn(1000))

		nodes := GetDefaultBackends(backend)

		config := Config{
			Backend:      backend,
			BackendNodes: nodes,
			Prefix:       prefix,
		}
		storeClient, err := New(config)
		assert.NoError(t, err)

		storeClient.Delete("/", true)

		values := map[string]interface{}{
			"subkey1": map[string]interface{}{
				"subkey1sub1": "subsubvalue1",
				"subkey1sub2": "subsubvalue2",
			},
		}

		err = storeClient.Put("testkey", values, true)
		assert.NoError(t, err)

		val, getErr := storeClient.Get("testkey", true)
		assert.NoError(t, getErr)
		assert.Equal(t, values, val)

		//test update

		values2 := map[string]interface{}{
			"subkey1": map[string]interface{}{
				"subkey1sub3": "subsubvalue3",
			},
		}

		err = storeClient.Put("testkey", values2, false)
		assert.NoError(t, err)

		values3 := map[string]interface{}{
			"subkey1": map[string]interface{}{
				"subkey1sub1": "subsubvalue1",
				"subkey1sub2": "subsubvalue2",
				"subkey1sub3": "subsubvalue3",
			},
		}

		val, getErr = storeClient.Get("testkey", true)
		assert.NoError(t, getErr)
		assert.Equal(t, values3, val)

		//test replace

		err = storeClient.Put("testkey", values2, true)
		assert.NoError(t, err)

		val, getErr = storeClient.Get("testkey", true)
		assert.NoError(t, getErr)
		assert.Equal(t, values2, val)

		assert.NoError(t, storeClient.Delete("/", true))
	}
}

func TestClientPutJSON(t *testing.T) {
	for _, backend := range backendNodes {
		println("Test backend: ", backend)

		prefix := fmt.Sprintf("/prefix%v", rand.Intn(1000))

		nodes := GetDefaultBackends(backend)

		config := Config{
			Backend:      backend,
			BackendNodes: nodes,
			Prefix:       prefix,
		}
		storeClient, err := New(config)
		assert.NoError(t, err)

		storeClient.Delete("/", true)

		jsonVal := []byte(`
			{"subkey1":
				{
					"subkey1sub1":"subsubvalue1",
					"subkey1sub2": "subsubvalue2"
				}
			}
		`)
		var values interface{}
		err = json.Unmarshal(jsonVal, &values)
		assert.NoError(t, err)

		err = storeClient.Put("testkey", values, true)
		assert.NoError(t, err)

		val, getErr := storeClient.Get("testkey", true)
		assert.NoError(t, getErr)
		assert.Equal(t, values, val)

		//test update

		jsonVal2 := []byte(`
			{"subkey1":
				{
					"subkey1sub3":"subsubvalue3"
				}
			}
		`)

		var values2 interface{}
		err = json.Unmarshal(jsonVal2, &values2)
		assert.NoError(t, err)

		err = storeClient.Put("testkey", values2, false)
		assert.NoError(t, err)

		values3 := map[string]interface{}{
			"subkey1": map[string]interface{}{
				"subkey1sub1": "subsubvalue1",
				"subkey1sub2": "subsubvalue2",
				"subkey1sub3": "subsubvalue3",
			},
		}

		val, getErr = storeClient.Get("testkey", true)
		assert.NoError(t, getErr)
		assert.Equal(t, values3, val)

		//test replace

		err = storeClient.Put("testkey", values2, true)
		assert.NoError(t, err)

		val, getErr = storeClient.Get("testkey", true)
		assert.NoError(t, getErr)
		assert.Equal(t, values2, val)

		assert.NoError(t, storeClient.Delete("/", true))
	}
}

func TestClientSetMaxOps(t *testing.T) {
	//TODO for etcd3 batch update max ops
}

func TestClientSync(t *testing.T) {

	for _, backend := range backendNodes {
		println("Test backend: ", backend)

		prefix := fmt.Sprintf("/prefix%v", rand.Intn(1000))

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
		assert.NoError(t, err)

		storeClient.Delete("/", true)
		//assert.NoError(t, err)

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

		storeClient.Delete("/", true)
	}
}

func TestMapping(t *testing.T) {

	prefix := fmt.Sprintf("/prefix%v", rand.Intn(1000))

	for _, backend := range backendNodes {
		println("Test backend: ", backend)
		nodes := GetDefaultBackends(backend)

		config := Config{
			Backend:      backend,
			BackendNodes: nodes,
			Prefix:       prefix,
		}
		storeClient, err := New(config)
		assert.NoError(t, err)
		mappings := make(map[string]interface{})
		for i := 0; i < 10; i++ {
			ip := fmt.Sprintf("192.168.1.%v", i)
			mapping := map[string]string{
				"instance": fmt.Sprintf("/instances/%v", i),
				"config":   fmt.Sprintf("/configs/%v", i),
			}
			mappings[ip] = mapping
		}
		storeClient.PutMapping("/", mappings, true)

		val, err := storeClient.GetMapping("/", true)
		assert.NoError(t, err)
		m, mok := val.(map[string]interface{})
		assert.True(t, mok)
		assert.True(t, m["192.168.1.0"] != nil)

		ip := fmt.Sprintf("192.168.1.%v", 1)
		nodePath := "/" + ip + "/" + "instance"
		storeClient.PutMapping(nodePath, "/instances/new1", true)
		time.Sleep(1000 * time.Millisecond)
		val, err = storeClient.GetMapping(nodePath, false)
		assert.NoError(t, err)
		assert.Equal(t, "/instances/new1", val)
	}
}

func TestMappingSync(t *testing.T) {

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
		assert.NoError(t, err)

		metastore := store.New()

		//for test init sync.

		for i := 0; i < 10; i++ {
			ip := fmt.Sprintf("192.168.1.%v", i)
			mapping := map[string]string{
				"instance": fmt.Sprintf("/instances/%v", i),
				"config":   fmt.Sprintf("/configs/%v", i),
			}
			storeClient.PutMapping(ip, mapping, true)
		}
		time.Sleep(1000 * time.Millisecond)
		storeClient.SyncMapping(metastore, stopChan)
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

		for i := 10; i < 20; i++ {
			ip := fmt.Sprintf("192.168.1.%v", i)
			mapping := map[string]string{
				"instance": fmt.Sprintf("/instances/%v", i),
				"config":   fmt.Sprintf("/configs/%v", i),
			}
			storeClient.PutMapping(ip, mapping, true)
		}
		time.Sleep(1000 * time.Millisecond)
		for i := 10; i < 20; i++ {
			ip := fmt.Sprintf("192.168.1.%v", i)
			val, ok := metastore.Get(ip)
			assert.True(t, ok)
			mapVal, mok := val.(map[string]interface{})
			assert.True(t, mok)
			path := mapVal["instance"]
			assert.Equal(t, path, fmt.Sprintf("/instances/%v", i))
		}
		ip := fmt.Sprintf("192.168.1.%v", 1)
		nodePath := ip + "/" + "instance"
		storeClient.PutMapping(nodePath, "/instances/new1", true)
		time.Sleep(1000 * time.Millisecond)
		val, ok := metastore.Get(nodePath)
		assert.True(t, ok)
		assert.Equal(t, "/instances/new1", val)
	}
}

func FillTestData(storeClient StoreClient) map[string]string {
	testData := make(map[string]interface{})
	for i := 0; i < 10; i++ {
		ci := make(map[string]string)
		for j := 0; j < 10; j++ {
			ci[fmt.Sprintf("%v", j)] = fmt.Sprintf("%v-%v", i, j)
		}
		testData[fmt.Sprintf("%v", i)] = ci
	}
	err := storeClient.Put("/", testData, true)
	if err != nil {
		log.Error("SetValues error", err.Error())
	}
	return flatmap.Flatten(testData)
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
		storeClient.Put(key, newVal, true)
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
	storeClient.Delete(key, false)
	delete(testData, key)
	return key
}

func ValidTestData(t *testing.T, testData map[string]string, metastore store.Store) {
	for k, v := range testData {
		storeVal, _ := metastore.Get(k)
		assert.Equal(t, v, storeVal)
	}
}
