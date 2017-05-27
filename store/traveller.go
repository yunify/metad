package store

import (
	"encoding/json"
	"strings"
)

type AccessMode int
type EnterResult int

const (
	AccessModeNil       = AccessMode(-1)
	AccessModeForbidden = AccessMode(0)
	AccessModeRead      = AccessMode(1)
)

type AccessRule struct {
	Path string
	Mode AccessMode
}

type Traveller interface {
	// Enter path's node, return is success.
	Enter(path string) bool
	// Back to parent node
	Back()
	// BackStep back multi step
	BackStep(step int)
	// Back to root node
	BackToRoot()
	// GetValue get current node value, if node is dir, will return a map contains children's value, otherwise return node.Value
	GetValue() interface{}
	// Close release traveller
	Close()
	// GetVersion get store version.
	GetVersion() int64
}

type accessNode struct {
	Name     string
	Mode     AccessMode
	parent   *accessNode
	Children []*accessNode
}

func (n *accessNode) HasChildren() bool {
	return len(n.Children) > 0
}

func (n *accessNode) GetChildren(name string, strict bool) *accessNode {
	var wildcardNode *accessNode
	for _, c := range n.Children {
		if name == c.Name {
			return c
		}
		if !strict && c.Name == "*" {
			wildcardNode = c
		}
	}
	return wildcardNode
}

type accessTree struct {
	root *accessNode
}

func (t *accessTree) Json() string {
	b, _ := json.MarshalIndent(t.root, "", "  ")
	return string(b)
}

func newAccessTree(rules []AccessRule) *accessTree {
	root := &accessNode{
		Name:   "/",
		Mode:   AccessModeNil,
		parent: nil,
	}
	tree := &accessTree{
		root: root,
	}
	for _, rule := range rules {
		p := rule.Path
		curr := root
		if p != "/" {
			components := strings.Split(p, "/")
			for _, component := range components {
				if component == "" {
					continue
				}
				child := curr.GetChildren(component, true)
				if child == nil {
					child = &accessNode{Name: component, Mode: AccessModeNil, parent: curr}
					curr.Children = append(curr.Children, child)
				}
				curr = child
			}
		}
		curr.Mode = rule.Mode
	}
	return tree
}

type stackElement struct {
	node *accessNode
	mode AccessMode
}

type travellerStack struct {
	backend []*stackElement
}

func (s *travellerStack) Push(v *stackElement) {
	s.backend = append(s.backend, v)
}

func (s *travellerStack) Pop() *stackElement {
	l := len(s.backend)
	if l == 0 {
		return nil
	}
	e := s.backend[l-1]
	s.backend = s.backend[:l-1]
	return e
}

func (s *travellerStack) Clean() {
	s.backend = []*stackElement{}
}

type nodeTraveller struct {
	store          *store
	access         *accessTree
	currNode       *node
	currAccessNode *accessNode
	currMode       AccessMode
	stack          travellerStack
}

func newTraveller(store *store, rules []AccessRule) Traveller {
	accessTree := newAccessTree(rules)
	store.worldLock.RLock()
	return &nodeTraveller{store: store, access: accessTree, currNode: store.Root, currAccessNode: accessTree.root, currMode: accessTree.root.Mode}
}

func (t *nodeTraveller) Enter(path string) bool {
	if t.store == nil {
		panic("illegal status: access a closed traveller.")
	}
	if path == "/" {
		return t.enter(path)
	} else {
		components := strings.Split(path, "/")
		step := 0
		for _, component := range components {
			if component == "" {
				continue
			}
			if !t.enter(component) {
				t.BackStep(step)
				return false
			}
			step = step + 1
		}
		return true
	}
}

func (t *nodeTraveller) enter(node string) bool {
	if node == "/" {
		return true
	}
	n := t.currNode.GetChild(node)
	if n == nil {
		return false
	}
	var an *accessNode
	if t.currAccessNode != nil {
		an = t.currAccessNode.GetChildren(node, false)
	}
	result := false
	if an != nil {
		// if an HasChildren, means exist other rule for future access
		if an.HasChildren() || an.Mode >= AccessModeRead {
			result = true
		}
	} else {
		if t.currMode >= AccessModeRead {
			result = true
		}
	}

	if result {
		t.stack.Push(&stackElement{node: t.currAccessNode, mode: t.currMode})
		t.currNode = n
		t.currAccessNode = an
		if t.currAccessNode != nil && t.currAccessNode.Mode != AccessModeNil {
			t.currMode = t.currAccessNode.Mode
		}

	}
	return result
}

func (t *nodeTraveller) Back() {
	if t.store == nil {
		panic("illegal status: access a closed traveller.")
	}
	if t.currNode.IsRoot() {
		panic("illegal status")
	}
	e := t.stack.Pop()
	if e == nil {
		panic("illegal status")
	}
	t.currNode = t.currNode.parent
	t.currMode = e.mode
	t.currAccessNode = e.node
}

func (t *nodeTraveller) BackStep(step int) {
	for i := 0; i < step; i++ {
		t.Back()
	}
}

func (t *nodeTraveller) BackToRoot() {
	if t.store == nil {
		panic("illegal status: access a closed traveller.")
	}
	t.stack.Clean()
	t.currNode = t.store.Root
	t.currAccessNode = t.access.root
	t.currMode = t.currAccessNode.Mode
}

func (t *nodeTraveller) GetValue() interface{} {
	if t.store == nil {
		panic("illegal status: access a closed traveller.")
	}
	if t.currNode == nil {
		panic("illegal status.")
	}
	if t.currNode.IsDir() {
		values := make(map[string]interface{})
		for k, node := range t.currNode.Children {
			if !t.Enter(node.Name) {
				continue
			}
			v := t.GetValue()
			t.Back()
			m, isMap := v.(map[string]interface{})
			// skip empty dir.
			if isMap && len(m) == 0 {
				continue
			}
			values[k] = v
		}
		return values
	} else {
		return t.currNode.Value
	}
}

func (t *nodeTraveller) GetVersion() int64 {
	if t.store == nil {
		panic("illegal status: access a closed traveller.")
	}
	return t.store.Version()
}

func (t *nodeTraveller) Close(){
	if t.store == nil {
		panic("illegal status: access a closed traveller.")
	}
	t.store = nil
	t.access = nil
	t.currAccessNode = nil
	t.currNode = nil
	t.store.worldLock.RUnlock()
}