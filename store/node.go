package store

import (
	"fmt"
	"path"
)

type node struct {
	path string

	parent *node

	value    string           // for key-value pair
	children map[string]*node // for directory

	store Store // A reference to the store this node is attached to.
}

func newKV(store Store, nodePath string, value string, parent *node) *node {
	return &node{
		path:     nodePath,
		parent:   parent,
		children: nil,
		value:    value,
		store:    store,
	}
}

func newDir(store Store, nodePath string, parent *node) *node {
	return &node{
		path:     nodePath,
		parent:   parent,
		children: make(map[string]*node),
		store:    store,
	}
}

// IsHidden function checks if the node is a hidden node. A hidden node
// will begin with '_'
// A hidden node will not be shown via get command under a directory
// For example if we have /foo/_hidden and /foo/notHidden, get "/foo"
// will only return /foo/notHidden
func (n *node) IsHidden() bool {
	_, name := path.Split(n.path)

	return name[0] == '_'
}

// IsDir function checks whether the node is a dir.
func (n *node) IsDir() bool {
	return n.children != nil
}

// AsDir convert node to dir
func (n *node) AsDir() {
	if !n.IsDir() {
		n.children = make(map[string]*node)
	}
}

func (n *node) AsLeaf() {
	if n.IsDir() {
		n.children = nil
	}
}

// Read function gets the value of the node.
func (n *node) Read() string {
	return n.value
}

// Write function set the value of the node to the given value.
func (n *node) Write(value string) {
	n.value = value
}

// List function return a slice of nodes under the receiver node.
func (n *node) List() []*node {

	if !n.IsDir() {
		return make([]*node, 0)
	}

	nodes := make([]*node, len(n.children))

	i := 0
	for _, node := range n.children {
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

	child, ok := n.children[name]

	if ok {
		return child
	}

	return nil
}

func (n *node) ChildrenCount() int {
	if n.IsDir() {
		return len(n.children)
	}
	return 0
}

// Add function adds a node to the receiver node.
func (n *node) Add(child *node) {
	if n.children == nil {
		n.children = make(map[string]*node)
	}
	_, name := path.Split(child.path)
	n.children[name] = child
}

func (n *node) AddChildren(children map[string]interface{}) {
	for k, v := range children {

		switch v := v.(type) {
		case map[string]interface{}:
			child := newDir(n.store, path.Join(n.path, k), n)
			n.Add(child)
			child.AddChildren(v)
		default:
			value := fmt.Sprintf("%v", v)
			child := newKV(n.store, path.Join(n.path, k), value, n)
			n.Add(child)
		}
	}
}

// Remove function remove the node.
func (n *node) Remove(callback func(path string)) {
	_, name := path.Split(n.path)
	if n.parent != nil && n.parent.children[name] == n {
		delete(n.parent.children, name)

		if callback != nil {
			callback(n.path)
		}

		// if parent children is empty, try to remove parent or covert to leaf node .
		if n.parent.ChildrenCount() == 0 {
			if n.parent.value == "" {
				n.parent.Remove(callback)
			} else {
				n.parent.AsLeaf()
			}
		}
	}
}

func (n *node) RemoveChildren(callback func(path string)) {
	if n.IsDir() {
		for _, node := range n.children {
			node.Remove(callback)
		}
	}
}

// Return node value, if node is dir, will return a map contains children's value, otherwise return n.Value
func (n *node) GetValue() interface{} {
	if n.IsDir() {
		values := make(map[string]interface{})
		for k, node := range n.children {
			values[k] = node.GetValue()
		}
		return values
	} else {
		return n.value
	}
}
