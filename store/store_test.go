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

	fmt.Printf("%v", val)

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

	fmt.Printf("%v", val)

	val, ok = store.Get("/clusters/1/ip")
	assert.True(t, ok)
	assert.Equal(t, "192.168.0.1", val)

}

func TestStoreSet(t *testing.T) {
	store := New()
	store.Set("/cl-fctugrij/cmd/i-tasp99rb/a/b/c/d/e/f/g/id", false, "iTQsRC5MqeBxaoUJSH3dMkf5e8OxMF8JxH1j")
	store.Set("/cl-fctugrij/cmd/i-tasp99rb/a/b/c/d/e/f/g/id", false, "")
}
