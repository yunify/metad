package internal

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/yunify/metadata-proxy/backends"
	"github.com/yunify/metadata-proxy/store"
	"math/rand"
	"testing"
)

func FillTestData(prefix string, storeClient backends.StoreClient) map[string]string {
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
