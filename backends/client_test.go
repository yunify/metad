// Copyright 2018 Yunify Inc. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

package backends

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yunify/metad/log"
	"github.com/yunify/metad/store"
	"github.com/yunify/metad/util/flatmap"
)

var (
	backendNodes = []string{
		"etcdv3",
		"local",
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

		assert.NoError(t, storeClient.Delete("/", true))

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

		assert.NoError(t, storeClient.Delete("/", true))

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

		assert.NoError(t, storeClient.Delete("/", true))

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
				},
			 "int":9663676416,
			 "bool":true,
			 "float":1.1111111
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
			"int":   "9663676416",
			"bool":  "true",
			"float": "1.1111111",
		}

		val, getErr = storeClient.Get("testkey", true)
		assert.NoError(t, getErr)
		assert.Equal(t, values3, val)

		//test replace

		err = storeClient.Put("testkey", values2, true)
		assert.NoError(t, err)

		values4 := map[string]interface{}{
			"subkey1": map[string]interface{}{
				"subkey1sub3": "subsubvalue3",
			},
			"int":   "9663676416",
			"bool":  "true",
			"float": "1.1111111",
		}

		val, getErr = storeClient.Get("testkey", true)
		assert.NoError(t, getErr)
		assert.Equal(t, values4, val)

		assert.NoError(t, storeClient.Delete("/", true))
	}
}

func TestClientNoPrefix(t *testing.T) {
	for _, backend := range backendNodes {
		println("Test backend: ", backend)

		prefix := ""

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

		assert.NoError(t, storeClient.Delete("/", true))

		metastore := store.New()
		storeClient.Sync(metastore, stopChan)

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

		mappings := map[string]interface{}{
			"192.168.1.1": map[string]interface{}{
				"key": "/subkey1/subkey1sub1",
			},
		}
		err = storeClient.PutMapping("/", mappings, true)

		mappings2, merr := storeClient.GetMapping("/", true)
		assert.NoError(t, merr)
		assert.Equal(t, mappings, mappings2)

		time.Sleep(1000 * time.Millisecond)

		// mapping data should not sync to metadata
		_, val = metastore.Get("/_metad")
		assert.Nil(t, val)

		assert.NoError(t, storeClient.Delete("/", true))
		assert.NoError(t, getErr)

		val, getErr = storeClient.Get("testkey", true)
		assert.NoError(t, getErr)
		assert.Equal(t, 0, len(val.(map[string]interface{})))

		// delete data "/" should not delete mapping
		mappings2, merr = storeClient.GetMapping("/", true)
		assert.NoError(t, merr)
		assert.Equal(t, mappings, mappings2)
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
		time.Sleep(2000 * time.Millisecond)
		ValidTestData(t, testData, metastore, backend)

		RandomUpdate(testData, storeClient, 10)
		time.Sleep(1000 * time.Millisecond)
		ValidTestData(t, testData, metastore, backend)

		deletedKey := RandomDelete(testData, storeClient)
		time.Sleep(1000 * time.Millisecond)
		ValidTestData(t, testData, metastore, backend)

		_, val := metastore.Get(deletedKey)
		assert.Nil(t, val)

		storeClient.Delete("/", true)
	}
}

func TestMapping(t *testing.T) {
	for _, backend := range backendNodes {
		println("Test backend: ", backend)
		prefix := fmt.Sprintf("/prefix%v", rand.Intn(1000))
		group := fmt.Sprintf("/group%v", rand.Intn(1000))
		nodes := GetDefaultBackends(backend)

		config := Config{
			Backend:      backend,
			BackendNodes: nodes,
			Prefix:       prefix,
			Group:        group,
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
		storeClient.Delete("/", true)
		storeClient.DeleteMapping("/", true)
	}
}

func TestMappingSync(t *testing.T) {

	for _, backend := range backendNodes {
		prefix := fmt.Sprintf("/prefix%v", rand.Intn(1000))
		group := fmt.Sprintf("/group%v", rand.Intn(1000))
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
			Group:        group,
		}
		storeClient, err := New(config)
		assert.NoError(t, err)

		mappingstore := store.New()

		//for test init sync.

		for i := 0; i < 5; i++ {
			ip := fmt.Sprintf("192.168.1.%v", i)
			mapping := map[string]string{
				"instance": fmt.Sprintf("/instances/%v", i),
				"config":   fmt.Sprintf("/configs/%v", i),
			}
			storeClient.PutMapping(ip, mapping, true)
		}
		time.Sleep(1000 * time.Millisecond)
		storeClient.SyncMapping(mappingstore, stopChan)
		time.Sleep(1000 * time.Millisecond)

		for i := 0; i < 5; i++ {
			ip := fmt.Sprintf("192.168.1.%v", i)
			_, val := mappingstore.Get(ip)
			mapVal, mok := val.(map[string]interface{})
			assert.True(t, mok)
			path := mapVal["instance"]
			assert.Equal(t, path, fmt.Sprintf("/instances/%v", i))
		}

		for i := 5; i < 10; i++ {
			ip := fmt.Sprintf("192.168.1.%v", i)
			mapping := map[string]string{
				"instance": fmt.Sprintf("/instances/%v", i),
				"config":   fmt.Sprintf("/configs/%v", i),
			}
			storeClient.PutMapping(ip, mapping, true)
		}
		time.Sleep(1000 * time.Millisecond)

		for i := 5; i < 10; i++ {
			ip := fmt.Sprintf("192.168.1.%v", i)
			_, val := mappingstore.Get(ip)
			mapVal, mok := val.(map[string]interface{})
			assert.True(t, mok)
			path := mapVal["instance"]
			assert.Equal(t, path, fmt.Sprintf("/instances/%v", i))
		}
		ip := fmt.Sprintf("192.168.1.%v", 1)
		nodePath := ip + "/" + "instance"
		storeClient.PutMapping(nodePath, "/instances/new1", true)
		time.Sleep(1000 * time.Millisecond)
		_, val := mappingstore.Get(nodePath)
		assert.Equal(t, "/instances/new1", val)
		storeClient.Delete("/", true)
		storeClient.DeleteMapping("/", true)
	}
}

func TestAccessRule(t *testing.T) {
	for _, backend := range backendNodes {
		stopChan := make(chan bool)
		defer func() {
			stopChan <- true
		}()
		storeClient := NewTestClient(backend)

		accessStore := store.NewAccessStore()
		storeClient.SyncAccessRule(accessStore, stopChan)

		rules := map[string][]store.AccessRule{
			"192.168.1.1": {
				{Path: "/clusters", Mode: store.AccessModeForbidden},
				{Path: "/clusters/cl-1", Mode: store.AccessModeRead},
			},
			"192.168.1.2": {
				{Path: "/clusters", Mode: store.AccessModeForbidden},
				{Path: "/clusters/cl-2", Mode: store.AccessModeRead},
			},
		}
		var rulesGet map[string][]store.AccessRule
		err := storeClient.PutAccessRule(rules)
		assert.NoError(t, err)

		rulesGet, err = storeClient.GetAccessRule()
		assert.NoError(t, err)
		assert.Equal(t, rules, rulesGet)

		time.Sleep(1000 * time.Millisecond)
		assert.NotNil(t, accessStore.Get("192.168.1.1"))

		err = storeClient.DeleteAccessRule([]string{"192.168.1.2"})
		assert.NoError(t, err)

		rulesGet, err = storeClient.GetAccessRule()
		assert.NoError(t, err)
		_, ok := rulesGet["192.168.1.2"]
		assert.False(t, ok)

		rules2 := map[string][]store.AccessRule{
			"192.168.1.3": {
				{Path: "/clusters", Mode: store.AccessModeForbidden},
				{Path: "/clusters/cl-3", Mode: store.AccessModeRead},
			},
		}
		err = storeClient.PutAccessRule(rules2)
		assert.NoError(t, err)

		time.Sleep(1000 * time.Millisecond)

		assert.Nil(t, accessStore.Get("192.168.1.2"))
		assert.NotNil(t, accessStore.Get("192.168.1.3"))

		err = storeClient.DeleteAccessRule([]string{"192.168.1.1", "192.168.1.2", "192.168.1.3"})
		assert.NoError(t, err)
	}
}

func NewTestClient(backend string) StoreClient {
	prefix := fmt.Sprintf("/prefix%v", rand.Intn(1000))
	group := fmt.Sprintf("/group%v", rand.Intn(1000))
	println("Test backend: ", backend)
	nodes := GetDefaultBackends(backend)

	config := Config{
		Backend:      backend,
		BackendNodes: nodes,
		Prefix:       prefix,
		Group:        group,
	}
	storeClient, err := New(config)
	if err != nil {
		panic(err)
	}
	return storeClient
}

func FillTestData(storeClient StoreClient) map[string]string {
	testData := make(map[string]interface{})
	for i := 0; i < 5; i++ {
		ci := make(map[string]string)
		for j := 0; j < 5; j++ {
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

func ValidTestData(t *testing.T, testData map[string]string, metastore store.Store, backend string) {
	for k, v := range testData {
		_, storeVal := metastore.Get(k)
		assert.Equal(t, v, storeVal, "valid data fail for backend %s", backend)
	}
}
