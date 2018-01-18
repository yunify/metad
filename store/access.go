// Copyright 2018 Yunify Inc. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

package store

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"sync"
)

type AccessMode int

const (
	begin               = AccessMode(-2)
	AccessModeNil       = AccessMode(-1)
	AccessModeForbidden = AccessMode(0)
	AccessModeRead      = AccessMode(1)
	end                 = AccessMode(2)
)

func CheckAccessMode(mode AccessMode) bool {
	if mode <= begin || mode >= end {
		return false
	}
	return true
}

func CheckAccessRules(rules []AccessRule) error {
	keys := make(map[string]*struct{}, len(rules))
	for _, r := range rules {
		if !CheckAccessMode(r.Mode) {
			return fmt.Errorf("Invalid AccessMode [%v]", r.Mode)
		}
		if _, ok := keys[r.Path]; ok {
			return fmt.Errorf("AccessRule path [%s] repeat define.", r.Path)
		}
	}
	return nil
}

type AccessRule struct {
	Path string     `json:"path"`
	Mode AccessMode `json:"mode"`
}

func MarshalAccessRule(rules []AccessRule) string {
	b, _ := json.Marshal(rules)
	return string(b)
}

func UnmarshalAccessRule(data string) ([]AccessRule, error) {
	rules := []AccessRule{}
	err := json.Unmarshal([]byte(data), &rules)
	return rules, err
}

type accessNode struct {
	Name     string
	Mode     AccessMode
	parent   *accessNode
	Children []*accessNode
}

func (n *accessNode) Path() string {
	if n.parent == nil {
		return n.Name
	} else {
		return path.Join(n.parent.Path(), n.Name)
	}
}

func (n *accessNode) HasChild() bool {
	return len(n.Children) > 0
}

func (n *accessNode) GetChild(name string, strict bool) *accessNode {
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

type AccessStore interface {
	Get(host string) AccessTree
	GetAccessRule(hosts []string) map[string][]AccessRule
	Put(host string, rules []AccessRule)
	Puts(rules map[string][]AccessRule)
	Delete(host string)
}

func NewAccessStore() AccessStore {
	return &accessStore{m: make(map[string]AccessTree)}
}

type accessStore struct {
	m    map[string]AccessTree
	lock sync.RWMutex
}

func (s *accessStore) Delete(host string) {
	s.lock.Lock()
	delete(s.m, host)
	s.lock.Unlock()
}

func (s *accessStore) Get(host string) AccessTree {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.m[host]
}

func (s *accessStore) Put(host string, rules []AccessRule) {
	s.lock.Lock()
	s.m[host] = NewAccessTree(rules)
	s.lock.Unlock()
}

func (s *accessStore) Puts(rules map[string][]AccessRule) {
	s.lock.Lock()
	for k, v := range rules {
		s.m[k] = NewAccessTree(v)
	}
	s.lock.Unlock()
}

func (s *accessStore) GetAccessRule(hosts []string) map[string][]AccessRule {
	s.lock.RLock()
	defer s.lock.RUnlock()
	result := map[string][]AccessRule{}
	if len(hosts) == 0 {
		for k, v := range s.m {
			result[k] = v.ToAccessRule()
		}
	} else {
		for _, host := range hosts {
			if host == "" {
				continue
			}
			t, ok := s.m[host]
			if ok {
				result[host] = t.ToAccessRule()
			}
		}
	}
	return result
}

type AccessTree interface {
	GetRoot() *accessNode
	ToAccessRule() []AccessRule
	Json() string
}

type accessTree struct {
	Root *accessNode
}

func (t *accessTree) GetRoot() *accessNode {
	return t.Root
}

func (t *accessTree) ToAccessRule() []AccessRule {
	rules := []AccessRule{}
	rules = t.toAccessRule(t.Root, rules)
	return rules
}

func (t *accessTree) toAccessRule(node *accessNode, rules []AccessRule) []AccessRule {
	if node.Mode != AccessModeNil {
		rules = append(rules, AccessRule{Path: node.Path(), Mode: node.Mode})
	}
	if node.HasChild() {
		for _, child := range node.Children {
			rules = t.toAccessRule(child, rules)
		}
	}
	return rules
}

func (t *accessTree) Json() string {
	b, _ := json.MarshalIndent(t.Root, "", "  ")
	return string(b)
}

func NewAccessTree(rules []AccessRule) AccessTree {
	root := &accessNode{
		Name:   "/",
		Mode:   AccessModeNil,
		parent: nil,
	}
	tree := &accessTree{
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
				child := curr.GetChild(component, true)
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
