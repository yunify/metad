package metadata

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/yunify/metadata-proxy/backends"
	"github.com/yunify/metadata-proxy/log"
	"github.com/yunify/metadata-proxy/store"
	"github.com/yunify/metadata-proxy/util/flatmap"
	"math/rand"
	"testing"
	"time"
)

func init() {
	log.SetLevel("debug")
	rand.Seed(int64(time.Now().Nanosecond()))
}

var (
	backend      = "etcdv3"
	backendNodes = []string{
		//"etcd",
		"etcdv3",
	}
	maxNode = 10
)

func TestMetarepo(t *testing.T) {

	prefix := fmt.Sprintf("/prefix%v", rand.Intn(1000))

	nodes := backends.GetDefaultBackends(backend)

	config := backends.Config{
		Backend:      backend,
		BackendNodes: nodes,
		Prefix:       prefix,
	}
	storeClient, err := backends.New(config)
	assert.NoError(t, err)

	storeClient.Delete("/", true)
	//assert.NoError(t, err)

	metarepo := New(false, storeClient)

	metarepo.StartSync()

	testData := FillTestData(storeClient)
	time.Sleep(1000 * time.Millisecond)
	ValidTestData(t, testData, metarepo.data)

	val, ok := metarepo.Get("192.168.0.1", "/nodes/0")
	assert.True(t, ok)
	assert.NotNil(t, val)

	mapVal, mok := val.(map[string]interface{})
	assert.True(t, mok)

	_, mok = mapVal["name"]
	assert.True(t, mok)

	storeClient.Delete("/nodes/0", true)

	if backend == "etcd" {
		//TODO etcd v2 current not support watch children delete. so try resync

		metarepo.ReSync()
	}
	time.Sleep(1000 * time.Millisecond)
	_, ok = metarepo.Get("192.168.0.1", "/nodes/0")
	assert.False(t, ok)

	storeClient.Delete("/", true)
}

func TestMetarepoSelf(t *testing.T) {
	prefix := fmt.Sprintf("/prefix%v", rand.Intn(1000))

	nodes := backends.GetDefaultBackends(backend)

	config := backends.Config{
		Backend:      backend,
		BackendNodes: nodes,
		Prefix:       prefix,
	}
	storeClient, err := backends.New(config)
	assert.NoError(t, err)

	storeClient.Delete("/", true)

	metarepo := New(false, storeClient)
	metarepo.DeleteMapping("/")

	metarepo.StartSync()

	testData := FillTestData(storeClient)
	time.Sleep(1000 * time.Millisecond)
	ValidTestData(t, testData, metarepo.data)

	key := "node"
	mappings := make(map[string]interface{})
	for i := 0; i < maxNode; i++ {
		ip := fmt.Sprintf("192.168.1.%v", i)
		mapping := map[string]interface{}{
			key:     fmt.Sprintf("/nodes/%v", i),
			"nodes": "/",
		}
		mappings[ip] = mapping
	}
	// batch update
	err = metarepo.UpdateMapping("/", mappings, true)
	assert.NoError(t, err)
	time.Sleep(1000 * time.Millisecond)

	//test mapping get
	mappings2, ok := metarepo.GetMapping("/")
	assert.True(t, ok)
	assert.Equal(t, mappings, mappings2)

	// test GetSelf
	time.Sleep(1000 * time.Millisecond)
	p := rand.Intn(maxNode)
	ip := fmt.Sprintf("192.168.1.%v", p)

	val, ok := metarepo.GetSelf(ip, "/")
	mapVal, mok := val.(map[string]interface{})

	assert.True(t, mok)
	assert.NotNil(t, mapVal[key])

	val, ok = metarepo.GetSelf(ip, "/node/name")
	assert.True(t, ok)
	assert.Equal(t, fmt.Sprintf("node%v", p), val)

	//test date delete
	storeClient.Delete(fmt.Sprintf("/nodes/%v/name", p), false)

	if backend == "etcd" {
		//etcd v2 current not support watch children's children delete. so try resync
		metarepo.ReSync()
	}
	time.Sleep(1000 * time.Millisecond)
	val, ok = metarepo.GetSelf(ip, "/node/name")
	assert.False(t, ok)
	assert.Nil(t, val)

	expectMapping0 := map[string]interface{}{
		"node":  fmt.Sprintf("/nodes/%v", p),
		"nodes": "/",
	}

	// test update replace(false)
	err = metarepo.UpdateMapping(ip, map[string]interface{}{"node2": "/nodes/2"}, false)
	assert.NoError(t, err)

	expectMapping1 := map[string]interface{}{
		"node":  fmt.Sprintf("/nodes/%v", p),
		"nodes": "/",
		"node2": "/nodes/2",
	}
	time.Sleep(1000 * time.Millisecond)
	mapping, ok := metarepo.GetMapping(fmt.Sprintf("/%s", ip))
	assert.True(t, ok)
	assert.Equal(t, expectMapping1, mapping)

	// test update key
	err = metarepo.UpdateMapping(ip+"/node3", "/nodes/3", false)
	assert.NoError(t, err)

	expectMapping2 := map[string]interface{}{
		"node":  fmt.Sprintf("/nodes/%v", p),
		"nodes": "/",
		"node2": "/nodes/2",
		"node3": "/nodes/3",
	}
	time.Sleep(1000 * time.Millisecond)
	mapping, ok = metarepo.GetMapping(fmt.Sprintf("/%s", ip))
	assert.True(t, ok)
	assert.Equal(t, expectMapping2, mapping)

	// test delete mapping
	metarepo.DeleteMapping(ip + "/node3")
	time.Sleep(1000 * time.Millisecond)
	mapping, ok = metarepo.GetMapping(fmt.Sprintf("/%s", ip))
	assert.True(t, ok)
	assert.Equal(t, expectMapping1, mapping)

	// test update replace(true)
	err = metarepo.UpdateMapping(ip, expectMapping0, true)
	assert.NoError(t, err)
	time.Sleep(1000 * time.Millisecond)
	mapping, ok = metarepo.GetMapping(fmt.Sprintf("/%s", ip))
	assert.True(t, ok)
	assert.Equal(t, expectMapping0, mapping)

	storeClient.Delete("/", true)
	metarepo.DeleteMapping("/")
}

func TestMetarepoRoot(t *testing.T) {

	prefix := fmt.Sprintf("/prefix%v", rand.Intn(1000))

	nodes := backends.GetDefaultBackends(backend)

	config := backends.Config{
		Backend:      backend,
		BackendNodes: nodes,
		Prefix:       prefix,
	}
	storeClient, err := backends.New(config)
	assert.NoError(t, err)

	storeClient.Delete("/", true)

	FillTestData(storeClient)

	metarepo := New(false, storeClient)

	metarepo.DeleteMapping("/")

	metarepo.StartSync()
	time.Sleep(1000 * time.Millisecond)

	ip := "192.168.1.0"
	mapping := make(map[string]interface{})
	mapping["node"] = "/nodes/0"
	err = metarepo.UpdateMapping(ip, mapping, true)
	assert.NoError(t, err)

	time.Sleep(1000 * time.Millisecond)
	val, ok := metarepo.Get(ip, "/")
	assert.True(t, ok)
	mapVal, mok := val.(map[string]interface{})
	assert.True(t, mok)
	selfVal := mapVal["self"]
	assert.NotNil(t, selfVal)
	assert.True(t, len(mapVal) > 1)

	metarepo.SetOnlySelf(true)

	val, ok = metarepo.Get(ip, "/")
	mapVal = val.(map[string]interface{})
	selfVal = mapVal["self"]
	assert.NotNil(t, selfVal)
	assert.True(t, len(mapVal) == 1)

	storeClient.Delete("/", true)
	metarepo.DeleteMapping("/")
}

func FillTestData(storeClient backends.StoreClient) map[string]string {
	nodes := make(map[string]interface{})
	for i := 0; i < maxNode; i++ {
		node := make(map[string]interface{})
		node["name"] = fmt.Sprintf("node%v", i)
		node["ip"] = fmt.Sprintf("192.168.1.%v", i)
		nodes[fmt.Sprintf("%v", i)] = node
	}
	testData := map[string]interface{}{
		"nodes": nodes,
	}
	err := storeClient.Set("/", testData, true)
	if err != nil {
		log.Error("SetValues error", err.Error())
		panic(err)
	}
	return flatmap.Flatten(testData)
}

func ValidTestData(t *testing.T, testData map[string]string, metastore store.Store) {
	for k, v := range testData {
		storeVal, _ := metastore.Get(k)
		assert.Equal(t, v, storeVal)
	}
}
