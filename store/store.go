package store

import (
	"context"
	"fmt"
	"github.com/yunify/metad/atomic"
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
	// currentVersion int64 and
	// a string (nodePath is a leaf node) or
	// a map[string]interface{} (nodePath is dir)
	Get(ctx context.Context, nodePath string) (int64, interface{})
	// Put value can be a map[string]interface{} or string
	Put(ctx context.Context, nodePath string, value interface{})
	Delete(ctx context.Context, nodePath string)
	// PutBulk value should be a flatmap
	PutBulk(ctx context.Context, nodePath string, value map[string]string)
	Watch(ctx context.Context, nodePath string, buf int) Watcher
	// Clean clean the nodePath's node
	Clean(ctx context.Context, nodePath string)
	// Json output store as json
	Json() string
	// Version return store's current version
	Version() int64
	// Destroy the store
	Destroy()
}

type store struct {
	Root      *node
	version   atomic.AtomicLong
	worldLock sync.RWMutex // stop the world lock
	cleanChan chan string
}

func New() Store {
	s := newStore()
	return s
}

func newStore() *store {
	s := new(store)
	s.version = atomic.AtomicLong(int64(0))
	s.Root = newDir(s, "/", nil, VisibilityLevelPublic)
	s.cleanChan = make(chan string, 100)
	go func() {
		for {
			select {
			case nodePath, ok := <-s.cleanChan:
				if ok {
					s.worldLock.Lock()
					ctx := WithVisibility(nil, VisibilityLevelPrivate)
					node := s.internalGet(ctx, nodePath)
					if node != nil {
						node.Clean()
					}
					s.worldLock.Unlock()
				} else {
					return
				}
			}
		}
	}()
	return s
}

// Get returns a path value.
func (s *store) Get(ctx context.Context, nodePath string) (currentVersion int64, val interface{}) {

	s.worldLock.RLock()
	defer s.worldLock.RUnlock()
	currentVersion = s.version.Get()
	val = nil

	nodePath = path.Clean(path.Join("/", nodePath))

	n := s.internalGet(ctx, nodePath)
	if n != nil {
		val = n.GetValue(ctx)
		m, mok := val.(map[string]interface{})
		// treat empty dir as not found result.
		if mok && len(m) == 0 && !n.IsRoot() {
			val = nil
		}
	}
	return
}

// Put creates or update the node at nodePath, value should a map[string]interface{} or a string
func (s *store) Put(ctx context.Context, nodePath string, value interface{}) {
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

func (s *store) PutBulk(ctx context.Context, nodePath string, values map[string]string) {
	s.worldLock.Lock()
	defer s.worldLock.Unlock()
	s.internalPutBulk(nodePath, values)
}

// Delete deletes the node at the given path.
func (s *store) Delete(ctx context.Context, nodePath string) {

	s.worldLock.Lock()
	defer s.worldLock.Unlock()

	nodePath = path.Clean(path.Join("/", nodePath))

	n := s.internalGet(ctx, nodePath)
	if n == nil {
		// if the node does not exist, treat as success
		return
	}
	s.version.IncrementAndGet()
	n.Remove()
}

func (s *store) Watch(ctx context.Context, nodePath string, buf int) Watcher {
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
			n = newDir(s, nodeName, d, VisibilityLevelPublic)
		}
	}
	return n.Watch(ctx, buf)
}

func (s *store) Json() string {
	return s.Root.Json()
}

func (s *store) Version() int64 {
	return s.version.Get()
}

func (s *store) Clean(ctx context.Context, nodePath string) {
	select {
	case s.cleanChan <- nodePath:
	default:
		println("drop clean node:", nodePath)
		break
	}

}

func (s *store) Destroy() {
	s.worldLock.Lock()
	defer s.worldLock.Unlock()
	close(s.cleanChan)
	s.Root = nil
}

// walk walks all the nodePath and apply the walkFunc on each directory
func (s *store) walk(nodePath string, walkFunc func(prev *node, component string) *node) *node {
	components := strings.Split(nodePath, "/")

	curr := s.Root

	for i := 1; i < len(components); i++ {
		if len(components[i]) == 0 {
			// ignore empty string
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

	s.version.IncrementAndGet()

	// nodePath is "/", just ignore put value.
	if nodePath == "/" {
		return s.Root
	}
	dirName, orgNodeName := path.Split(nodePath)

	// walk through the nodePath, create dirs and get the last directory node
	d := s.walk(dirName, s.checkDir)

	// skip empty node name.
	if orgNodeName == "" {
		return d
	}

	nodeName, vLevel := ParseVisibility(orgNodeName)

	n := d.GetChild(nodeName)

	if n != nil {
		n.Write(value)
		if vLevel != VisibilityLevelNone {
			n.SetVisibility(vLevel)
		}
		return n
	}

	if vLevel == VisibilityLevelNone {
		vLevel = VisibilityLevelPublic
	}

	n = newKV(s, nodeName, value, d, vLevel)

	return n
}

func (s *store) internalPutBulk(nodePath string, values map[string]string) {
	for k, v := range values {
		key := util.AppendPathPrefix(k, nodePath)
		s.internalPut(key, v)
	}
}

// InternalGet gets the node of the given nodePath.
func (s *store) internalGet(ctx context.Context, nodePath string) *node {

	walkFunc := func(parent *node, name string) *node {
		if parent == nil {
			return nil
		}

		if !parent.IsDir() {
			return nil
		}

		child := parent.GetChild(name)
		if child != nil {
			vlevel, ok := ctx.Value(VisibilityKey).(VisibilityLevel)
			if !ok {
				vlevel = VisibilityLevelPublic
			}
			if vlevel < child.visibility {
				return nil
			}
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
func (s *store) checkDir(parent *node, orgDirName string) *node {
	// skip empty node name.
	if orgDirName == "" {
		return parent
	}
	dirName, vLevel := ParseVisibility(orgDirName)

	node := parent.GetChild(dirName)

	if node != nil {
		if vLevel != VisibilityLevelNone {
			node.SetVisibility(vLevel)
		}
		return node
	}
	if vLevel == VisibilityLevelNone {
		vLevel = VisibilityLevelPublic
	}
	n := newDir(s, dirName, parent, vLevel)

	return n
}
