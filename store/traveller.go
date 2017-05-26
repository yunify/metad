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
	// can Enter node
	Enter(node string) bool
	// Back to parent dir
	Back()

	GetValue() interface{}
}

type pathTree struct {
	root *pathNode
}

func (t *pathTree) Json() string {
	b, _ := json.MarshalIndent(t.root, "", "  ")
	return string(b)
}

type pathNode struct {
	Name     string
	Mode     AccessMode
	parent   *pathNode
	Children []*pathNode
}

func (n *pathNode) HasChildren() bool {
	return len(n.Children) > 0
}

func (n *pathNode) GetChildren(name string, strict bool) *pathNode {
	var wildcardNode *pathNode
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

func newNodeTraveller(store *store, rules []AccessRule) Traveller {
	return &nodeTraveller{store: store, tree: buildPathTree(rules)}
}

func buildPathTree(rules []AccessRule) *pathTree {
	root := &pathNode{
		Name:   "/",
		Mode:   AccessModeNil,
		parent: nil,
	}
	tree := &pathTree{
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
					child = &pathNode{Name: component, Mode: AccessModeNil, parent: curr}
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
	pathNode *pathNode
	mode     AccessMode
}

type travellerStack struct {
	backend []interface{}
}

func (s *travellerStack) Push(v interface{}) {
	s.backend = append(s.backend, v)
}

func (s *travellerStack) Pop() interface{} {
	l := len(s.backend)
	if l == 0 {
		return nil
	}
	e := s.backend[l-1]
	s.backend = s.backend[:l-1]
	return e
}

type nodeTraveller struct {
	store        *store
	tree         *pathTree
	currNode     *node
	currPathNode *pathNode
	currMode     AccessMode
	stack        travellerStack
}

func (t *nodeTraveller) Enter(dir string) bool {
	if dir == "/" {
		if t.currNode == nil {
			t.stack.Push(&stackElement{pathNode: t.currPathNode, mode: t.currMode})
			t.currNode = t.store.Root
			t.currPathNode = t.tree.root
			t.currMode = t.currPathNode.Mode
			return true
		}
		return false
	}
	if t.currNode == nil {
		return false
	}
	n := t.currNode.GetChild(dir)
	if n == nil {
		return false
	}
	//if !n.IsDir() {
	//	return EnterNotDir
	//}

	var p *pathNode
	if t.currPathNode != nil {
		p = t.currPathNode.GetChildren(dir, false)
	}
	result := false
	if p != nil {
		// if p HasChildren, means exist other rule for future access
		if p.HasChildren() || p.Mode >= AccessModeRead {
			result = true
		}
	} else {
		if t.currMode >= AccessModeRead {
			result = true
		}
	}

	if result {
		t.stack.Push(&stackElement{pathNode: t.currPathNode, mode: t.currMode})
		t.currNode = n
		t.currPathNode = p
		if t.currPathNode != nil && t.currPathNode.Mode != AccessModeNil {
			t.currMode = t.currPathNode.Mode
		}

	}
	return result
}

func (t *nodeTraveller) Back() {
	if t.currNode == nil || t.currNode.IsRoot() {
		panic("illegal status")
	}
	e := t.stack.Pop()
	if e == nil {
		panic("illegal status")
	}
	ele := e.(*stackElement)
	t.currNode = t.currNode.parent
	t.currMode = ele.mode
	t.currPathNode = ele.pathNode
}

func (t *nodeTraveller) GetValue() interface{} {
	if t.currNode == nil {
		panic("illegal status.")
	}
	if t.currNode.IsDir() {
		values := make(map[string]interface{})
		for k, node := range t.currNode.Children {
			eresult := t.Enter(node.Name)
			if !eresult {
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
