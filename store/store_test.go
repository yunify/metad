package store

import (
	"github.com/stretchr/testify/assert"
	"testing"
	//"github.com/stretchr/testify/suite"
	"fmt"
)

func TestStoreBasic(t *testing.T) {
	store := New()

	val, ok := store.Get("/foo")
	assert.False(t, ok)
	assert.Nil(t, val)

	store.Set("/foo", false, "bar")

	val, ok = store.Get("/foo")
	assert.True(t, ok)
	assert.Equal(t, "bar", val)

	store.Delete("/foo")

	val, ok = store.Get("/foo")
	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestStoreDir(t *testing.T) {
	store := New()

	store.Set("/foo", true, "")

	store.Set("/foo/foo1", true, "")

	val, ok := store.Get("/foo/foo1")
	assert.True(t, ok)
	mapVal, mok := val.(map[string]interface{})
	assert.True(t, mok)
	assert.Equal(t, 0, len(mapVal))

	store.Set("/foo/foo1/key1", false, "val1")
	val, ok = store.Get("/foo/foo1/key1")
	assert.True(t, ok)
	assert.Equal(t, "val1", val)

	store.Delete("/foo/foo1")

	val, ok = store.Get("/foo/foo1")
	assert.False(t, ok)
	assert.Nil(t, val)

}

func TestStoreBulk(t *testing.T) {
	store := New()

	//store.Set("/clusters", true, nil)
	values := make(map[string]string)
	for i := 1; i <= 10; i++ {
		values[fmt.Sprintf("/clusters/%v/ip", i)] = fmt.Sprintf("192.168.0.%v", i)
		values[fmt.Sprintf("/clusters/%v/name", i)] = fmt.Sprintf("cluster-%v", i)
	}
	store.SetBulk("/", values)

	val, ok := store.Get("/clusters/10")
	assert.True(t, ok)

	val, ok = store.Get("/clusters/1/ip")
	assert.True(t, ok)
	assert.Equal(t, "192.168.0.1", val)

}

func TestStoreSets(t *testing.T) {
	store := New()

	values := make(map[string]interface{})
	for i := 1; i <= 10; i++ {
		values[fmt.Sprintf("%v", i)] = map[string]interface{}{
			"ip":   fmt.Sprintf("192.168.0.%v", i),
			"name": fmt.Sprintf("cluster-%v", i),
		}
	}
	store.Sets("/clusters", values)

	val, ok := store.Get("/clusters/10")
	assert.True(t, ok)

	val, ok = store.Get("/clusters/1/ip")
	assert.True(t, ok)
	assert.Equal(t, "192.168.0.1", val)

}

func TestStoreNodeToDirPanic(t *testing.T) {
	store := New()
	// first set a node value.
	store.Set("/nodes/6", false, "node6")
	// create pre node's child's child, will cause panic.
	store.Set("/nodes/6/label/key1", false, "value1")

	v, _ := store.Get("/nodes/6")
	_, mok := v.(map[string]interface{})
	assert.True(t, mok)

	v, _ = store.Get("/nodes/6/label/key1")
	assert.Equal(t, "value1", v)
}

func TestStoreRemoveEmptyParent(t *testing.T) {
	store := New()

	// if dir has children, dir's text value will be hidden.
	store.Set("/nodes/6", false, "node6")
	store.Set("/nodes/6/label/key1", false, "value1")

	store.Delete("/nodes/6/label/key1")

	v, ok := store.Get("/nodes/6/label")
	assert.False(t, ok)

	// if dir's children been deleted, and dir has text value ,dir will become a leaf node.
	v, ok = store.Get("/nodes/6")
	assert.True(t, ok)
	assert.Equal(t, "node6", v)

	// when delete leaf node, empty parent dir will been auto delete.
	store.Set("/nodes/7/label/key1", false, "value1")
	store.Delete("/nodes/7/label/key1")

	_, ok = store.Get("/nodes/7")
	assert.False(t, ok)
}
