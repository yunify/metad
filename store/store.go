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
	// return a string (nodePath is a leaf node) or
	// a map[string]interface{} (nodePath is dir)
	Get(nodePath string) (interface{}, bool)
	// Put value can be a map[string]interface{} or string
	Put(nodePath string, value interface{})
	Delete(nodePath string)
	// PutBulk value should be a flatmap
	PutBulk(nodePath string, value map[string]string)
}

type store struct {
	Root      *node
	worldLock sync.RWMutex // stop the world lock
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
func (s *store) Get(nodePath string) (interface{}, bool) {

	s.worldLock.RLock()
	defer s.worldLock.RUnlock()

	nodePath = path.Clean(path.Join("/", nodePath))

	n := s.internalGet(nodePath)
	if n != nil {
		return n.GetValue(), true
	}

	return nil, false
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

	// we do not allow the user to change "/", remove "/" equals remove "/*"
	if nodePath == "/" {
		s.Root.RemoveChildren(nil)
		return
	}

	n := s.internalGet(nodePath)
	if n == nil { // if the node does not exist, treat as success
		return
	}
	n.Remove(nil)
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

	dirName, nodeName := path.Split(nodePath)

	// walk through the nodePath, create dirs and get the last directory node
	d := s.walk(dirName, s.checkDir)
	n := d.GetChild(nodeName)

	// force will try to replace an existing file
	if n != nil {
		n.value = value
		return n
	}

	n = newKV(s, nodePath, value, d)
	d.Add(n)

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
	node := parent.GetChild(dirName)

	if node != nil {
		return node
	}

	n := newDir(s, path.Join(parent.path, dirName), parent)
	parent.Add(n)
	return n
}
