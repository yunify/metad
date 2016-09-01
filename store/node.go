package store

import (
	"container/list"
	"encoding/json"
	"errors"
	"path"
)

type node struct {
	Name string `json:"name"`

	parent *node

	watchers *list.List

	Value    string           `json:"value"`    // for key-value pair
	Children map[string]*node `json:"children"` // for directory

	store *store // A reference to the store this node is attached to.
}

func newKV(store *store, nodeName string, value string, parent *node) *node {
	if len(nodeName) == 0 {
		panic(errors.New("nodeName can not be emtpy."))
	}
	n := &node{
		Name:     nodeName,
		parent:   parent,
		watchers: nil,
		Children: nil,
		Value:    value,
		store:    store,
	}
	parent.Add(n)
	n.Notify(Update)
	return n
}

func newDir(store *store, nodeName string, parent *node) *node {
	if len(nodeName) == 0 {
		panic(errors.New("nodeName can not be emtpy."))
	}
	n := &node{
		Name:     nodeName,
		parent:   parent,
		watchers: nil,
		Children: make(map[string]*node),
		store:    store,
	}
	if parent != nil {
		parent.Add(n)
	}
	return n
}

func (n *node) Path() string {
	if n.parent == nil {
		return n.Name
	} else {
		return path.Join(n.parent.Path(), n.Name)
	}
}

func (n *node) RelativePath(end *node) string {
	if n.parent == nil || n == end {
		return "/"
	} else {
		return path.Join(n.parent.RelativePath(end), n.Name)
	}
}

func (n *node) IsRoot() bool {
	return n.parent == nil
}

// IsHidden function checks if the node is a hidden node. A hidden node
// will begin with '_'
// A hidden node will not be shown via get command under a directory
// For example if we have /foo/_hidden and /foo/notHidden, get "/foo"
// will only return /foo/notHidden
func (n *node) IsHidden() bool {
	return n.Name[0] == '_'
}

// IsDir function checks whether the node is a dir.
func (n *node) IsDir() bool {
	return n.Children != nil
}

// AsDir convert node to dir
func (n *node) AsDir() {
	if !n.IsDir() {
		n.Children = make(map[string]*node)
	}
	// treat convert leaf to dir as a delete.
	n.Notify(Delete)
}

func (n *node) AsLeaf() {
	if n.IsDir() {
		n.Children = nil
	}
	// treat convert dir to leaf as a update.
	n.Notify(Update)
}

// Read function gets the value of the node.
func (n *node) Read() string {
	return n.Value
}

// Write function set the value of the node to the given value.
func (n *node) Write(value string) {
	if n.IsRoot() {
		return
	}

	oldValue := n.Value
	n.Value = value
	if n.IsDir() {
		// if dir is empty, and set a text value ,so convert to leaf
		if n.ChildrenCount() == 0 {
			n.AsLeaf()
		}
	} else {
		if oldValue != value {
			n.Notify(Update)
		}
	}
}

// List function return a slice of nodes under the receiver node.
func (n *node) List() []*node {

	if !n.IsDir() {
		return make([]*node, 0)
	}

	nodes := make([]*node, len(n.Children))

	i := 0
	for _, node := range n.Children {
		nodes[i] = node
		i++
	}

	return nodes
}

// GetChild function returns the child node under the directory node.
// On success, it returns the file node
func (n *node) GetChild(name string) *node {
	if !n.IsDir() {
		return nil
	}

	child, ok := n.Children[name]

	if ok {
		return child
	}

	return nil
}

func (n *node) ChildrenCount() int {
	if n.IsDir() {
		return len(n.Children)
	}
	return 0
}

// Add function adds a node to the receiver node.
func (n *node) Add(child *node) {
	if !n.IsDir() {
		n.AsDir()
	}
	n.Children[child.Name] = child
}

// Remove function remove the node.
func (n *node) Remove() bool {

	if !n.IsDir() {
		// do not remove node has watcher
		if n.watchers != nil && n.watchers.Len() > 0 {
			n.Value = ""
			n.AsDir()
			return true
		}
		if n.parent != nil && n.parent.Children[n.Name] == n {
			delete(n.parent.Children, n.Name)
			// only leaf node trigger delete event.
			n.Notify(Delete)
			n.parent.Clean()
			return true
		}
		return false
	}

	// clear value
	n.Value = ""

	// retry to remove all children
	for _, node := range n.Children {
		node.Remove()
	}

	if n.parent != nil && n.parent.Children[n.Name] == n && n.ChildrenCount() == 0 && (n.watchers == nil || n.watchers.Len() == 0) {
		delete(n.parent.Children, n.Name)
		n.parent.Clean()
		return true
	}
	return false
}

// Clean empty dir
func (n *node) Clean() bool {
	if !n.IsDir() {
		return false
	}
	// if children is empty, try to remove  or covert to leaf node .
	if n.ChildrenCount() == 0 {
		if n.Value == "" {
			if n.watchers == nil || n.watchers.Len() == 0 {
				return n.Remove()
			}
		} else {
			n.AsLeaf()
			return true
		}
	}
	return false
}

// Return node value, if node is dir, will return a map contains children's value, otherwise return n.Value
func (n *node) GetValue() interface{} {
	if n.IsDir() {
		values := make(map[string]interface{})
		for k, node := range n.Children {
			//skip hidden node.
			if node.IsHidden() {
				continue
			}
			v := node.GetValue()
			m, isMap := v.(map[string]interface{})
			// skip empty dir.
			if isMap && len(m) == 0 {
				continue
			}
			values[k] = v
		}
		return values
	} else {
		return n.Value
	}
}

func (n *node) internalNotify(action string, eventNode *node) {
	if n.watchers != nil && n.watchers.Len() > 0 {
		event := newEvent(action, eventNode.RelativePath(n), eventNode.Value)
		for e := n.watchers.Front(); e != nil; e = e.Next() {
			w := e.Value.(Watcher)
			w.EventChan() <- event
		}
	}
	// pop up event.
	if n.parent != nil {
		n.parent.internalNotify(action, eventNode)
	}
}

func (n *node) Notify(action string) {
	n.internalNotify(action, n)
}

func (n *node) Watch() Watcher {
	if n.watchers == nil {
		n.watchers = list.New()
	}
	w := newWatcher(n)
	elem := n.watchers.PushBack(w)
	w.remove = func() {
		if w.removed { // avoid removing it twice
			return
		}
		w.removed = true
		n.watchers.Remove(elem)
		if n.watchers.Len() == 0 {
			n.Clean()
		}
	}

	return w
}

func (s *node) Json() string {
	b, _ := json.Marshal(s)
	return string(b)
}
