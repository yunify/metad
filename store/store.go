package store

import (
	"github.com/yunify/metadata-proxy/util"
	"github.com/yunify/metadata-proxy/util/flatmap"
	"path"
	"strings"
	"sync"
)

type Store interface {
	Get(nodePath string) (interface{}, bool)
	Set(nodePath string, dir bool, value string)
	Sets(nodePath string, values map[string]interface{})
	Delete(nodePath string)
	SetBulk(nodePath string, value map[string]string)
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

// Set creates or replace the node at nodePath, return old value
func (s *store) Set(nodePath string, dir bool, value string) {
	nodePath = path.Clean(path.Join("/", nodePath))

	s.worldLock.Lock()
	defer s.worldLock.Unlock()
	s.internalSet(nodePath, dir, value)
}

func (s *store) SetBulk(nodePath string, values map[string]string) {
	s.worldLock.Lock()
	defer s.worldLock.Unlock()
	s.internalSetBulk(nodePath, values)
}

func (s *store) internalSetBulk(nodePath string, values map[string]string) {
	for k, v := range values {
		key := util.AppendPathPrefix(k, nodePath)
		s.internalSet(key, false, v)
	}
}

func (s *store) Sets(nodePath string, values map[string]interface{}) {
	nodePath = path.Clean(path.Join("/", nodePath))

	s.worldLock.Lock()
	defer s.worldLock.Unlock()
	flatValues := flatmap.Flatten(values)
	s.internalSetBulk(nodePath, flatValues)
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

func (s *store) internalSet(nodePath string, dir bool, value string) *node {

	dirName, nodeName := path.Split(nodePath)

	// walk through the nodePath, create dirs and get the last directory node
	d := s.walk(dirName, s.checkDir)
	n := d.GetChild(nodeName)

	// force will try to replace an existing file
	if n != nil {
		if dir {
			n.AsDir()
		}
		n.value = value
		return n
	}

	if !dir { // create file

		n = newKV(s, nodePath, value, d)
	} else { // create directory

		n = newDir(s, nodePath, d)
	}

	d.Add(n)

	return n
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
