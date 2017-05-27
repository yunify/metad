package store

import (
	"encoding/json"
	"strings"
)

type AccessMode int

const (
	AccessModeNil       = AccessMode(-1)
	AccessModeForbidden = AccessMode(0)
	AccessModeRead      = AccessMode(1)
)

type AccessRule struct {
	Path string     `json:"path"`
	Mode AccessMode `json:"mode"`
}

type AccessNode struct {
	Name     string
	Mode     AccessMode
	parent   *AccessNode
	Children []*AccessNode
}

func (n *AccessNode) HasChildren() bool {
	return len(n.Children) > 0
}

func (n *AccessNode) GetChildren(name string, strict bool) *AccessNode {
	var wildcardNode *AccessNode
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

type AccessTree struct {
	Root *AccessNode
}

func (t *AccessTree) Json() string {
	b, _ := json.MarshalIndent(t.Root, "", "  ")
	return string(b)
}

func NewAccessTree(rules []AccessRule) *AccessTree {
	root := &AccessNode{
		Name:   "/",
		Mode:   AccessModeNil,
		parent: nil,
	}
	tree := &AccessTree{
		Root: root,
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
					child = &AccessNode{Name: component, Mode: AccessModeNil, parent: curr}
					curr.Children = append(curr.Children, child)
				}
				curr = child
			}
		}
		curr.Mode = rule.Mode
	}
	return tree
}
