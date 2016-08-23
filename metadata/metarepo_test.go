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
}

var (
	backendNodes = []string{
		//"etcd",
		"etcdv3",
	}
)

func TestMetarepo(t *testing.T) {

	prefix := fmt.Sprintf("/prefix%v", rand.Intn(1000))

	for _, backend := range backendNodes {

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

		val, ok := metarepo.Get("192.168.0.1", "/0")
		assert.True(t, ok)
		assert.NotNil(t, val)

		mapVal, mok := val.(map[string]interface{})
		assert.True(t, mok)

		_, mok = mapVal["0"]
		assert.True(t, mok)

		storeClient.Delete("/0/0", false)

		if backend == "etcd" {
			//TODO etcd v2 current not support watch children delete. so try resync

			metarepo.ReSync()
			time.Sleep(1000 * time.Millisecond)
		}

		_, ok = metarepo.Get("192.168.0.1", "/0/0")
		assert.False(t, ok)

		storeClient.Delete("/", true)
	}
}

func TestMetarepoSelf(t *testing.T) {

	prefix := fmt.Sprintf("/prefix%v", rand.Intn(1000))

	for _, backend := range backendNodes {

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
		time.Sleep(2000 * time.Millisecond)
		ValidTestData(t, testData, metarepo.data)

		val, ok := metarepo.Get("192.168.0.1", "/0")
		assert.True(t, ok)
		assert.NotNil(t, val)

		mapVal, mok := val.(map[string]interface{})
		assert.True(t, mok)

		_, mok = mapVal["0"]
		assert.True(t, mok)

		for i := 0; i < 10; i++ {
			ip := fmt.Sprintf("192.168.1.%v", i)
			mapping := map[string]string{}
			key := fmt.Sprintf("s%v", i)
			mapping[key] = fmt.Sprintf("/%v", i)
			metarepo.Register(ip, mapping)
		}
		time.Sleep(2000 * time.Millisecond)
		p := rand.Intn(10)
		ip := fmt.Sprintf("192.168.1.%v", p)

		val, ok = metarepo.GetSelf(ip, "/")
		mapVal, mok = val.(map[string]interface{})

		key := fmt.Sprintf("s%v", p)
		assert.True(t, mok)
		assert.NotNil(t, mapVal[key])

		val, ok = metarepo.GetSelf(ip, fmt.Sprintf("/s%v/%v/%v", p, p, p))
		assert.True(t, ok)
		assert.Equal(t, fmt.Sprintf("%v-%v-%v", p, p, p), val)

		storeClient.Delete("/0/0/0", false)

		if backend == "etcd" {
			//etcd v2 current not support watch children's children delete. so try resync
			metarepo.ReSync()
		}
		time.Sleep(1000 * time.Millisecond)
		val, ok = metarepo.GetSelf("192.168.1.0", "/s0/0/0")
		assert.False(t, ok)
		assert.Nil(t, val)

		//storeClient.Delete("/", true)
	}
}

func TestMetarepoRoot(t *testing.T) {

	prefix := fmt.Sprintf("/prefix%v", rand.Intn(1000))

	for _, backend := range backendNodes {

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
		metarepo.StartSync()
		time.Sleep(1000 * time.Millisecond)

		ip := "192.168.1.0"
		mapping := make(map[string]string)
		mapping["nodes"] = fmt.Sprintf("%s/1/0", prefix)
		metarepo.Register(ip, mapping)
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
	}
}

func FillTestData(storeClient backends.StoreClient) map[string]string {
	testData := make(map[string]interface{})
	for i := 0; i < 10; i++ {
		ci := make(map[string]interface{})
		for j := 0; j < 10; j++ {
			cj := make(map[string]string)
			for k := 0; k < 10; k++ {
				cj[fmt.Sprintf("%v", k)] = fmt.Sprintf("%v-%v-%v", i, j, k)
			}
			ci[fmt.Sprintf("%v", j)] = cj
		}
		testData[fmt.Sprintf("%v", i)] = ci
	}
	err := storeClient.SetValues("/", testData, true)
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
