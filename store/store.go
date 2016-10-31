package store

import (
	"fmt"
	"github.com/yunify/metad/util"
	"github.com/yunify/metad/util/flatmap"
	"path"
	"reflect"
	"strings"
	"sync"
)

type Store interface {
	// Get
	// return
	// currentVersion uint64 and
	// a string (nodePath is a leaf node) or
	// a map[string]interface{} (nodePath is dir)
	Get(nodePath string) (uint64, interface{})
	// Put value can be a map[string]interface{} or string
	Put(nodePath string, value interface{})
	Delete(nodePath string)
	// PutBulk value should be a flatmap
	PutBulk(nodePath string, value map[string]string)
	Watch(nodePath string) Watcher
	// Json output store as json
	Json() string
	// Version return store's current version
	Version() uint64
}

type store struct {
	Root        *node
	version     uint64
	worldLock   sync.RWMutex // stop the world lock
	watcherLock sync.RWMutex
}

func New() Store {
	s := newStore()
	return s
}

func newStore() *store {
	s := new(store)
	s.Root = newDir(s, "/", nil)
	return s
}

// Get returns a path value.
func (s *store) Get(nodePath string) (currentVersion uint64, val interface{}) {

	s.worldLock.RLock()
	defer s.worldLock.RUnlock()
	currentVersion = s.version
	val = nil

	nodePath = path.Clean(path.Join("/", nodePath))

	n := s.internalGet(nodePath)
	if n != nil {
		val = n.GetValue()
		m, mok := val.(map[string]interface{})
		// treat empty dir as not found result.
		if mok && len(m) == 0 && !n.IsRoot() {
			val = nil
		}
	}
	return
}

// Put creates or update the node at nodePath, value should a map[string]interface{} or a string
func (s *store) Put(nodePath string, value interface{}) {
	nodePath = path.Clean(path.Join("/", nodePath))

	s.worldLock.Lock()
	defer s.worldLock.Unlock()
	switch t := value.(type) {
	case map[string]interface{}, map[string]string, []interface{}:
		flatValues := flatmap.Flatten(t)
		s.internalPutBulk(nodePath, flatValues)
	case string:
		s.internalPut(nodePath, t)
	default:
		panic(fmt.Sprintf("Unsupport type: %s", reflect.TypeOf(t)))
	}
}

func (s *store) PutBulk(nodePath string, values map[string]string) {
	s.worldLock.Lock()
	defer s.worldLock.Unlock()
	s.internalPutBulk(nodePath, values)
}

// Delete deletes the node at the given path.
func (s *store) Delete(nodePath string) {

	s.worldLock.Lock()
	defer s.worldLock.Unlock()

	nodePath = path.Clean(path.Join("/", nodePath))

	n := s.internalGet(nodePath)
	if n == nil { // if the node does not exist, treat as success
		return
	}
	s.version = s.version + 1
	n.Remove()
}

func (s *store) Watch(nodePath string) Watcher {
	s.worldLock.Lock()
	defer s.worldLock.Unlock()
	var n *node
	if nodePath == "/" {
		n = s.Root
	} else {

		dirName, nodeName := path.Split(nodePath)

		// walk through the nodePath, create dirs and get the last directory node
		d := s.walk(dirName, s.checkDir)
		n = d.GetChild(nodeName)
		if n == nil {
			// if watch node not exist, create a empty dir.
			n = newDir(s, nodeName, d)
		}
	}
	return n.Watch()
}

func (s *store) Json() string {
	return s.Root.Json()
}

func (s *store) Version() uint64 {
	return s.version
}

// walk walks all the nodePath and apply the walkFunc on each directory
func (s *store) walk(nodePath string, walkFunc func(prev *node, component string) *node) *node {
	components := strings.Split(nodePath, "/")

	curr := s.Root

	for i := 1; i < len(components); i++ {
		if len(components[i]) == 0 { // ignore empty string
			return curr
		}

		curr = walkFunc(curr, components[i])
		if curr == nil {
			return nil
		}
	}

	return curr
}

func (s *store) internalPut(nodePath string, value string) *node {

	s.version = s.version + 1

	// nodePath is "/", just ignore put value.
	if nodePath == "/" {
		return s.Root
	}
	dirName, nodeName := path.Split(nodePath)

	// walk through the nodePath, create dirs and get the last directory node
	d := s.walk(dirName, s.checkDir)

	// skip empty node name.
	if nodeName == "" {
		return d
	}

	n := d.GetChild(nodeName)

	if n != nil {
		n.Write(value)
		return n
	}

	n = newKV(s, nodeName, value, d)
	return n
}

func (s *store) internalPutBulk(nodePath string, values map[string]string) {
	for k, v := range values {
		key := util.AppendPathPrefix(k, nodePath)
		s.internalPut(key, v)
	}
}

// InternalGet gets the node of the given nodePath.
func (s *store) internalGet(nodePath string) *node {

	walkFunc := func(parent *node, name string) *node {

		if !parent.IsDir() {
			return nil
		}

		child := parent.GetChild(name)
		if child != nil {
			return child
		}

		return nil
	}

	f := s.walk(nodePath, walkFunc)
	return f
}

// checkDir will check whether the component is a directory under parent node.
// If it is a directory, this function will return the pointer to that node.
// If it does not exist, this function will create a new directory and return the pointer to that node.
// If it is a file, this function will return error.
func (s *store) checkDir(parent *node, dirName string) *node {
	// skip empty node name.
	if dirName == "" {
		return parent
	}
	node := parent.GetChild(dirName)

	if node != nil {
		return node
	}

	n := newDir(s, dirName, parent)
	return n
}
