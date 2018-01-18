// Copyright 2018 Yunify Inc. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

package store

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStoreBasic(t *testing.T) {
	s := New()

	_, val := s.Get("/foo")
	assert.Nil(t, val)

	s.Put("/foo", "bar")

	//println(store.Json())

	_, val = s.Get("/foo")
	assert.Equal(t, "bar", val)

	s.Delete("/foo")

	_, val = s.Get("/foo")
	assert.Nil(t, val)
	s.Destroy()
}

func TestStoreDir(t *testing.T) {
	s := New()

	s.Put("/foo/foo1", "")

	_, val := s.Get("/foo")
	_, mok := val.(map[string]interface{})
	assert.True(t, mok)

	s.Put("/foo/foo1/key1", "val1")
	_, val = s.Get("/foo/foo1/key1")
	assert.Equal(t, "val1", val)

	s.Delete("/foo/foo1")

	_, val = s.Get("/foo/foo1")
	assert.Nil(t, val)
	s.Destroy()

}

func TestStoreBulk(t *testing.T) {
	s := New()

	//store.Set("/clusters", true, nil)
	values := make(map[string]string)
	for i := 1; i <= 10; i++ {
		values[fmt.Sprintf("/clusters/%v/ip", i)] = fmt.Sprintf("192.168.0.%v", i)
		values[fmt.Sprintf("/clusters/%v/name", i)] = fmt.Sprintf("cluster-%v", i)
	}
	s.PutBulk("/", values)

	_, val := s.Get("/clusters/10")

	_, val = s.Get("/clusters/1/ip")
	assert.Equal(t, "192.168.0.1", val)
	s.Destroy()

}

func TestStoreSets(t *testing.T) {
	s := New()

	values := make(map[string]interface{})
	for i := 1; i <= 10; i++ {
		values[fmt.Sprintf("%v", i)] = map[string]interface{}{
			"ip":   fmt.Sprintf("192.168.0.%v", i),
			"name": fmt.Sprintf("cluster-%v", i),
		}
	}
	s.Put("/clusters", values)

	_, val := s.Get("/clusters/10")

	_, val = s.Get("/clusters/1/ip")
	assert.Equal(t, "192.168.0.1", val)
	s.Destroy()

}

func TestStoreNodeToDirPanic(t *testing.T) {
	s := New()
	// first set a node value.
	s.Put("/nodes/6", "node6")
	// create pre node's child's child, will cause panic.
	s.Put("/nodes/6/label/key1", "value1")

	_, v := s.Get("/nodes/6")
	_, mok := v.(map[string]interface{})
	assert.True(t, mok)

	_, v = s.Get("/nodes/6/label/key1")
	assert.Equal(t, "value1", v)
	s.Destroy()
}

func TestStoreClean(t *testing.T) {
	s := New()

	// if dir has children, dir's text value will be hidden.
	s.Put("/nodes/6", "node6")
	s.Put("/nodes/6/label/key1", "value1")

	//println(store.Json())

	s.Delete("/nodes/6/label/key1")

	//println(store.Json())

	_, val := s.Get("/nodes/6/label")
	assert.Nil(t, val)

	// if dir's children been deleted, and dir has text value ,dir will become a leaf node.
	_, val = s.Get("/nodes/6")
	assert.Equal(t, "node6", val)

	// when delete leaf node, empty parent dir will been auto delete.
	s.Put("/nodes/7/label/key1", "value1")
	s.Delete("/nodes/7/label/key1")

	_, val = s.Get("/nodes/7")
	assert.Nil(t, val)
	s.Destroy()
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
	w := s.Watch("/nodes/6", 100)
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
	s2.worldLock.RLock()
	n := s2.internalGet("/nodes/6")
	s2.worldLock.RUnlock()
	assert.NotNil(t, n)

	w.Remove()

	//wait backend goroutine to clean
	time.Sleep(5 * time.Second)
	s2.worldLock.RLock()
	n = s2.internalGet("/nodes/6")
	s2.worldLock.RUnlock()
	assert.Nil(t, n)

	s2.worldLock.RLock()
	n = s2.internalGet("/nodes")
	s2.worldLock.RUnlock()
	assert.Nil(t, n)

	s.Destroy()
}

func TestWatchRoot(t *testing.T) {
	s := New()
	s.Put("/nodes/6/name", "node6")

	//watch root
	w := s.Watch("/", 100)
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
	s.Destroy()
}

func TestEmptyStore(t *testing.T) {
	s := newStore()
	_, val := s.Get("/")
	assert.Equal(t, 0, len(val.(map[string]interface{})))

	s.Put("/", "test")

	_, val = s.Get("/")
	assert.Equal(t, 0, len(val.(map[string]interface{})))

	w := s.Watch("/", 10)
	assert.NotNil(t, w)
	s.Delete("/")
	e := readEvent(w.EventChan())
	assert.Nil(t, e)

	w.Remove()
	s.Destroy()
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
	s.Destroy()

}

func TestConcurrentWatchAndPut(t *testing.T) {
	go func() {
		println(http.ListenAndServe("localhost:6060", nil))
	}()
	s := New()
	loop := 50000
	wg := sync.WaitGroup{}
	wg.Add(3)
	starter := sync.WaitGroup{}
	starter.Add(1)
	go func() {
		starter.Wait()
		for i := 0; i < loop; i++ {
			w := s.Watch("/nodes/1", 1000)
			go func() {
				for {
					select {
					case _, ok := <-w.EventChan():
						if ok {
							//println(e.Path, e.Action)
						} else {
							return
						}
					default:
						return
					}
				}
			}()
			w.Remove()
		}
		wg.Done()
	}()

	go func() {
		starter.Wait()
		for i := 0; i < loop; i++ {
			s.Put("/nodes/1/name", "n1")
			s.Delete("/nodes/1/name")
		}
		wg.Done()
	}()

	go func() {
		starter.Wait()
		for i := 0; i < loop; i++ {
			s.Get("/nodes/1")
		}
		wg.Done()
	}()
	starter.Done()
	wg.Wait()
	s.Destroy()
}
