package metadata

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/yunify/metad/backends"
	"github.com/yunify/metad/log"
	"github.com/yunify/metad/store"
	"github.com/yunify/metad/util/flatmap"
	"math/rand"
	"testing"
	"time"
)

func init() {
	log.SetLevel("debug")
	rand.Seed(int64(time.Now().Nanosecond()))
}

var (
	backend   = "local"
	maxNode   = 10
	sleepTime = 100 * time.Millisecond
)

func TestMetarepoData(t *testing.T) {

	prefix := fmt.Sprintf("/prefix%v", rand.Intn(1000))

	nodes := backends.GetDefaultBackends(backend)

	config := backends.Config{
		Backend:      backend,
		BackendNodes: nodes,
		Prefix:       prefix,
	}
	storeClient, err := backends.New(config)
	assert.NoError(t, err)

	metarepo := New(false, storeClient)
	metarepo.DeleteData("/")

	metarepo.StartSync()

	testData := FillTestData(metarepo)
	time.Sleep(sleepTime)
	ValidTestData(t, testData, metarepo.data)

	val, ok := metarepo.Root("192.168.0.1", "/nodes/0")
	assert.True(t, ok)
	assert.NotNil(t, val)

	mapVal, mok := val.(map[string]interface{})
	assert.True(t, mok)

	_, mok = mapVal["name"]
	assert.True(t, mok)

	metarepo.DeleteData("/nodes/0")

	if backend == "etcd" {
		//TODO etcd v2 current not support watch children delete. so try resync
		metarepo.ReSync()
	}
	time.Sleep(1000 * time.Millisecond)
	_, ok = metarepo.GetData("/nodes/0")
	assert.False(t, ok)

	subs := []string{"1", "3", "noexistkey"}
	//test batch delete
	err = metarepo.DeleteData("nodes", subs...)
	time.Sleep(1000 * time.Millisecond)
	assert.NoError(t, err)

	for _, sub := range subs {
		_, ok = metarepo.GetData("/nodes/" + sub)
		assert.False(t, ok)
	}

	_, ok = metarepo.GetData("/nodes/2")
	assert.True(t, ok)

	metarepo.DeleteData("/")
	metarepo.StopSync()
}

func TestMetarepoMapping(t *testing.T) {

	prefix := fmt.Sprintf("/prefix%v", rand.Intn(1000))
	group := fmt.Sprintf("/group%v", rand.Intn(1000))
	nodes := backends.GetDefaultBackends(backend)

	config := backends.Config{
		Backend:      backend,
		BackendNodes: nodes,
		Prefix:       prefix,
		Group:        group,
	}
	storeClient, err := backends.New(config)
	assert.NoError(t, err)

	metarepo := New(false, storeClient)
	metarepo.DeleteData("/")
	metarepo.DeleteMapping("/")

	metarepo.StartSync()

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
	err = metarepo.PutMapping("/", mappings, true)
	assert.NoError(t, err)
	time.Sleep(1000 * time.Millisecond)

	metarepo.DeleteMapping("/192.168.1.0")

	time.Sleep(1000 * time.Millisecond)
	_, ok := metarepo.GetMapping("/192.168.1.0")
	assert.False(t, ok)

	subs := []string{"192.168.1.1", "192.168.1.3", "noexistkey"}
	//test batch delete
	err = metarepo.DeleteMapping("/", subs...)
	time.Sleep(1000 * time.Millisecond)
	assert.NoError(t, err)

	for _, sub := range subs {
		_, ok = metarepo.GetMapping("/" + sub)
		assert.False(t, ok)
	}

	_, ok = metarepo.GetMapping("/192.168.1.2")
	assert.True(t, ok)

	p := rand.Intn(maxNode)
	ip := fmt.Sprintf("192.168.1.%v", p)

	expectMapping0 := map[string]interface{}{
		"node":  fmt.Sprintf("/nodes/%v", p),
		"nodes": "/",
	}

	// test update replace(false)
	err = metarepo.PutMapping(ip, map[string]interface{}{"node2": "/nodes/2"}, false)
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
	err = metarepo.PutMapping(ip+"/node3", "/nodes/3", false)
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
	err = metarepo.PutMapping(ip, expectMapping0, true)
	assert.NoError(t, err)
	time.Sleep(1000 * time.Millisecond)
	mapping, ok = metarepo.GetMapping(fmt.Sprintf("/%s", ip))
	assert.True(t, ok)
	assert.Equal(t, expectMapping0, mapping)

	metarepo.DeleteData("/")
	metarepo.DeleteMapping("/")
	metarepo.StopSync()
}

func TestMetarepoSelf(t *testing.T) {
	prefix := fmt.Sprintf("/prefix%v", rand.Intn(1000))
	group := fmt.Sprintf("/group%v", rand.Intn(1000))
	nodes := backends.GetDefaultBackends(backend)

	config := backends.Config{
		Backend:      backend,
		BackendNodes: nodes,
		Prefix:       prefix,
		Group:        group,
	}
	storeClient, err := backends.New(config)
	assert.NoError(t, err)

	metarepo := New(false, storeClient)

	metarepo.DeleteMapping("/")
	metarepo.DeleteData("/")

	metarepo.StartSync()

	testData := FillTestData(metarepo)
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
	err = metarepo.PutMapping("/", mappings, true)
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

	val, ok := metarepo.Self(ip, "/")
	mapVal, mok := val.(map[string]interface{})

	assert.True(t, mok)
	assert.NotNil(t, mapVal[key])

	val, ok = metarepo.Self(ip, "/node/name")
	assert.True(t, ok)
	assert.Equal(t, fmt.Sprintf("node%v", p), val)

	//test date delete
	metarepo.DeleteData(fmt.Sprintf("/nodes/%v/name", p))

	if backend == "etcd" {
		//etcd v2 current not support watch children's children delete. so try resync
		metarepo.ReSync()
	}
	time.Sleep(1000 * time.Millisecond)
	val, ok = metarepo.Self(ip, "/node/name")
	assert.False(t, ok)
	assert.Nil(t, val)

	//test mapping dir

	err = metarepo.PutMapping(ip, map[string]interface{}{
		"dir": map[string]interface{}{
			"n1": "/nodes/1",
			"n2": "/nodes/2",
		},
	}, false)

	time.Sleep(1000 * time.Millisecond)
	val, ok = metarepo.Self(ip, "/dir/n1/name")
	assert.True(t, ok)
	assert.Equal(t, "node1", val)

	metarepo.DeleteData("/")
	metarepo.DeleteMapping("/")
	metarepo.StopSync()
}

func TestMetarepoRoot(t *testing.T) {

	prefix := fmt.Sprintf("/prefix%v", rand.Intn(1000))
	group := fmt.Sprintf("/group%v", rand.Intn(1000))
	nodes := backends.GetDefaultBackends(backend)

	config := backends.Config{
		Backend:      backend,
		BackendNodes: nodes,
		Prefix:       prefix,
		Group:        group,
	}
	storeClient, err := backends.New(config)
	assert.NoError(t, err)

	metarepo := New(false, storeClient)

	metarepo.DeleteMapping("/")
	metarepo.DeleteData("/")

	FillTestData(metarepo)

	metarepo.StartSync()
	time.Sleep(1000 * time.Millisecond)

	ip := "192.168.1.0"
	mapping := make(map[string]interface{})
	mapping["node"] = "/nodes/0"
	err = metarepo.PutMapping(ip, mapping, true)
	assert.NoError(t, err)

	time.Sleep(1000 * time.Millisecond)
	val, ok := metarepo.Root(ip, "/")
	assert.True(t, ok)
	mapVal, mok := val.(map[string]interface{})
	assert.True(t, mok)
	selfVal := mapVal["self"]
	assert.NotNil(t, selfVal)
	assert.True(t, len(mapVal) > 1)

	metarepo.SetOnlySelf(true)

	val, ok = metarepo.Root(ip, "/")
	mapVal = val.(map[string]interface{})
	selfVal = mapVal["self"]
	assert.NotNil(t, selfVal)
	assert.True(t, len(mapVal) == 1)

	metarepo.DeleteData("/")
	metarepo.DeleteMapping("/")
	metarepo.StopSync()
}

func FillTestData(metarepo *MetadataRepo) map[string]string {
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
	err := metarepo.PutData("/", testData, true)
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
