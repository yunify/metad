package store

import (
	"fmt"
	"path"
)

type node struct {
	Path string

	Parent *node `json:"-"` // should not encode this field! avoid circular dependency.

	Value    string           // for key-value pair
	Children map[string]*node // for directory

	// A reference to the store this node is attached to.
	store Store
}

func newKV(store Store, nodePath string, value string, parent *node) *node {
	return &node{
		Path:     nodePath,
		Parent:   parent,
		Children: nil,
		Value:    value,
		store:    store,
	}
}

func newDir(store Store, nodePath string, parent *node) *node {
	return &node{
		Path:     nodePath,
		Parent:   parent,
		Children: make(map[string]*node),
		store:    store,
	}
}

// IsHidden function checks if the node is a hidden node. A hidden node
// will begin with '_'
// A hidden node will not be shown via get command under a directory
// For example if we have /foo/_hidden and /foo/notHidden, get "/foo"
// will only return /foo/notHidden
func (n *node) IsHidden() bool {
	_, name := path.Split(n.Path)

	return name[0] == '_'
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
}

// Read function gets the value of the node.
func (n *node) Read() string {
	return n.Value
}

// Write function set the value of the node to the given value.
func (n *node) Write(value string) {
	n.Value = value
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

// Add function adds a node to the receiver node.
func (n *node) Add(child *node) {
	if n.Children == nil {
		n.Children = make(map[string]*node)
	}
	_, name := path.Split(child.Path)
	n.Children[name] = child
}

func (n *node) AddChildren(children map[string]interface{}) {
	for k, v := range children {

		switch v := v.(type) {
		case map[string]interface{}:
			child := newDir(n.store, path.Join(n.Path, k), n)
			n.Add(child)
			child.AddChildren(v)
		default:
			value := fmt.Sprintf("%v", v)
			child := newKV(n.store, path.Join(n.Path, k), value, n)
			n.Add(child)
		}
	}
}

// Remove function remove the node.
func (n *node) Remove(callback func(path string)) {
	_, name := path.Split(n.Path)
	if n.Parent != nil && n.Parent.Children[name] == n {
		delete(n.Parent.Children, name)

		if callback != nil {
			callback(n.Path)
		}
	}
}

func (n *node) RemoveChildren(callback func(path string)) {
	if n.IsDir() {
		for _, node := range n.Children {
			node.Remove(callback)
		}
	}
}

// Return node value, if node is dir, will return a map contains children's value, otherwise return n.Value
func (n *node) getValue() interface{} {
	if n.IsDir() {
		values := make(map[string]interface{})
		for k, node := range n.Children {
			values[k] = node.getValue()
		}
		return values
	} else {
		return n.Value
	}
}
