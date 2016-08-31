package store

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestStoreBasic(t *testing.T) {
	store := New()

	val, ok := store.Get("/foo")
	assert.False(t, ok)
	assert.Nil(t, val)

	store.Put("/foo", "bar")

	//println(store.Json())

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

	store.Put("/foo/foo1", "")

	val, ok := store.Get("/foo")
	assert.True(t, ok)
	_, mok := val.(map[string]interface{})
	assert.True(t, mok)

	store.Put("/foo/foo1/key1", "val1")
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
	store.PutBulk("/", values)

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
	store.Put("/clusters", values)

	val, ok := store.Get("/clusters/10")
	assert.True(t, ok)

	val, ok = store.Get("/clusters/1/ip")
	assert.True(t, ok)
	assert.Equal(t, "192.168.0.1", val)

}

func TestStoreNodeToDirPanic(t *testing.T) {
	store := New()
	// first set a node value.
	store.Put("/nodes/6", "node6")
	// create pre node's child's child, will cause panic.
	store.Put("/nodes/6/label/key1", "value1")

	v, _ := store.Get("/nodes/6")
	_, mok := v.(map[string]interface{})
	assert.True(t, mok)

	v, _ = store.Get("/nodes/6/label/key1")
	assert.Equal(t, "value1", v)
}

func TestStoreClean(t *testing.T) {
	store := New()

	// if dir has children, dir's text value will be hidden.
	store.Put("/nodes/6", "node6")
	store.Put("/nodes/6/label/key1", "value1")

	//println(store.Json())

	store.Delete("/nodes/6/label/key1")

	//println(store.Json())

	v, ok := store.Get("/nodes/6/label")
	assert.False(t, ok)

	// if dir's children been deleted, and dir has text value ,dir will become a leaf node.
	v, ok = store.Get("/nodes/6")
	assert.True(t, ok)
	assert.Equal(t, "node6", v)

	// when delete leaf node, empty parent dir will been auto delete.
	store.Put("/nodes/7/label/key1", "value1")
	store.Delete("/nodes/7/label/key1")

	_, ok = store.Get("/nodes/7")
	assert.False(t, ok)
}

func TestWatch(t *testing.T) {
	s := New()
	//watch a no exist node
	w := s.Watch("/nodes/6")
	s.Put("/nodes/6", "node6")
	e := <-w.EventChan()
	assert.Equal(t, Update, e.Action)
	assert.Equal(t, "/nodes/6", e.Path)

	s.Put("/nodes/6/label/key1", "value1")

	e = <-w.EventChan()
	assert.Equal(t, Update, e.Action)
	assert.Equal(t, "/nodes/6/label/key1", e.Path)

	s.Put("/nodes/6/label/key1", "value2")

	e = <-w.EventChan()
	assert.Equal(t, Update, e.Action)
	assert.Equal(t, "/nodes/6/label/key1", e.Path)

	s.Delete("/nodes/6/label/key1")

	e = <-w.EventChan()
	assert.Equal(t, Delete, e.Action)
	assert.Equal(t, "/nodes/6/label/key1", e.Path)

	// when /nodes/6's children remove, it return to a leaf node.
	e = <-w.EventChan()
	assert.Equal(t, Update, e.Action)
	assert.Equal(t, "/nodes/6", e.Path)

	s.Delete("/nodes/6")

	e = <-w.EventChan()
	assert.Equal(t, Delete, e.Action)
	assert.Equal(t, "/nodes/6", e.Path)

	select {
	case e = <-w.EventChan():
	default:
		e = nil
	}
	// expect no more event.
	assert.Nil(t, e)

	s2 := s.(*store)
	n := s2.internalGet("/nodes/6")
	assert.NotNil(t, n)
	w.Remove()

	n = s2.internalGet("/nodes/6")
	assert.Nil(t, n)
}
