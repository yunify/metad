package store

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestStoreBasic(t *testing.T) {
	store := New()

	_, val := store.Get("/foo")
	assert.Nil(t, val)

	store.Put("/foo", "bar")

	//println(store.Json())

	_, val = store.Get("/foo")
	assert.Equal(t, "bar", val)

	store.Delete("/foo")

	_, val = store.Get("/foo")
	assert.Nil(t, val)
}

func TestStoreDir(t *testing.T) {
	store := New()

	store.Put("/foo/foo1", "")

	_, val := store.Get("/foo")
	_, mok := val.(map[string]interface{})
	assert.True(t, mok)

	store.Put("/foo/foo1/key1", "val1")
	_, val = store.Get("/foo/foo1/key1")
	assert.Equal(t, "val1", val)

	store.Delete("/foo/foo1")

	_, val = store.Get("/foo/foo1")
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

	_, val := store.Get("/clusters/10")

	_, val = store.Get("/clusters/1/ip")
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

	_, val := store.Get("/clusters/10")

	_, val = store.Get("/clusters/1/ip")
	assert.Equal(t, "192.168.0.1", val)

}

func TestStoreNodeToDirPanic(t *testing.T) {
	store := New()
	// first set a node value.
	store.Put("/nodes/6", "node6")
	// create pre node's child's child, will cause panic.
	store.Put("/nodes/6/label/key1", "value1")

	_, v := store.Get("/nodes/6")
	_, mok := v.(map[string]interface{})
	assert.True(t, mok)

	_, v = store.Get("/nodes/6/label/key1")
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

	_, val := store.Get("/nodes/6/label")
	assert.Nil(t, val)

	// if dir's children been deleted, and dir has text value ,dir will become a leaf node.
	_, val = store.Get("/nodes/6")
	assert.Equal(t, "node6", val)

	// when delete leaf node, empty parent dir will been auto delete.
	store.Put("/nodes/7/label/key1", "value1")
	store.Delete("/nodes/7/label/key1")

	_, val = store.Get("/nodes/7")
	assert.Nil(t, val)
}

func readEvent(ch chan *Event) *Event {
	var e *Event
	select {
	case e = <-ch:
		//println("readEvent", e)
	case <-time.After(1 * time.Second):
		//println("readEvent timeout")
	}
	return e
}

func TestWatch(t *testing.T) {
	s := New()
	//watch a no exist node
	w := s.Watch("/nodes/6")
	s.Put("/nodes/6", "node6")
	e := readEvent(w.EventChan())
	assert.Equal(t, Update, e.Action)
	assert.Equal(t, "/", e.Path)
	assert.Equal(t, "node6", e.Value)

	s.Put("/nodes/6/label/key1", "value1")

	// leaf node /nodes/6 convert to dir, tread as deleted.
	e = readEvent(w.EventChan())
	assert.Equal(t, Delete, e.Action)
	assert.Equal(t, "/", e.Path)

	e = readEvent(w.EventChan())
	assert.Equal(t, Update, e.Action)
	assert.Equal(t, "/label/key1", e.Path)
	assert.Equal(t, "value1", e.Value)

	s.Put("/nodes/6/label/key1", "value2")

	e = readEvent(w.EventChan())
	assert.Equal(t, Update, e.Action)
	assert.Equal(t, "/label/key1", e.Path)
	assert.Equal(t, "value2", e.Value)

	s.Delete("/nodes/6/label/key1")

	e = readEvent(w.EventChan())
	assert.Equal(t, Delete, e.Action)
	assert.Equal(t, "/label/key1", e.Path)

	// when /nodes/6's children remove, it return to a leaf node.
	e = readEvent(w.EventChan())
	assert.Equal(t, Update, e.Action)
	assert.Equal(t, "/", e.Path)

	s.Put("/nodes/6/name", "node6")
	s.Put("/nodes/6/ip", "192.168.1.1")

	e = readEvent(w.EventChan())
	assert.Equal(t, Delete, e.Action)
	assert.Equal(t, "/", e.Path)

	e = readEvent(w.EventChan())
	assert.Equal(t, Update, e.Action)
	assert.Equal(t, "/name", e.Path)
	e = readEvent(w.EventChan())
	assert.Equal(t, Update, e.Action)
	assert.Equal(t, "/ip", e.Path)

	s.Delete("/nodes/6")

	e = readEvent(w.EventChan())
	//println(e.Action,e.Path)
	assert.Equal(t, Delete, e.Action)
	assert.True(t, e.Path == "/name" || e.Path == "/ip")

	e = readEvent(w.EventChan())
	//println(e.Action,e.Path)
	assert.Equal(t, Delete, e.Action)
	assert.True(t, e.Path == "/name" || e.Path == "/ip")

	e = readEvent(w.EventChan())
	// expect no more event.
	assert.Nil(t, e)

	s2 := s.(*store)
	n := s2.internalGet("/nodes/6")
	assert.NotNil(t, n)

	w.Remove()

	n = s2.internalGet("/nodes/6")
	assert.Nil(t, n)

	n = s2.internalGet("/nodes")
	assert.Nil(t, n)
}

func TestWatchRoot(t *testing.T) {
	s := New()
	s.Put("/nodes/6/name", "node6")

	//watch root
	w := s.Watch("/")
	s.Put("/nodes/6/ip", "192.168.1.1")

	var e *Event
	e = readEvent(w.EventChan())
	assert.Equal(t, Update, e.Action)
	assert.Equal(t, "/nodes/6/ip", e.Path)

	s.Delete("/")

	e = readEvent(w.EventChan())
	//println(e.Action,e.Path)
	assert.Equal(t, Delete, e.Action)
	assert.True(t, e.Path == "/nodes/6/name" || e.Path == "/nodes/6/ip")

	e = readEvent(w.EventChan())
	//println(e.Action,e.Path)
	assert.Equal(t, Delete, e.Action)
	assert.True(t, e.Path == "/nodes/6/name" || e.Path == "/nodes/6/ip")

	e = readEvent(w.EventChan())
	// expect no more event.
	assert.Nil(t, e)
	w.Remove()
}

func TestEmptyStore(t *testing.T) {
	s := newStore()
	_, val := s.Get("/")
	assert.Equal(t, 0, len(val.(map[string]interface{})))

	s.Put("/", "test")

	_, val = s.Get("/")
	assert.Equal(t, 0, len(val.(map[string]interface{})))

	w := s.Watch("/")
	assert.NotNil(t, w)
	s.Delete("/")
	e := readEvent(w.EventChan())
	assert.Nil(t, e)

	w.Remove()
}

func TestBlankNode(t *testing.T) {
	s := newStore()
	s.Put("/", map[string]interface{}{
		"": map[string]interface{}{
			"": "blank_node",
		},
	})

	_, val := s.Get("/")
	assert.Equal(t, 0, len(val.(map[string]interface{})))

	s.Put("/test//node", "n1")
	_, val = s.Get("/test/node")
	assert.Equal(t, val, "n1")

}
