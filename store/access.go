package store

import (
	"encoding/json"
	"fmt"
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

type AccessNode struct {
	Name     string
	Mode     AccessMode
	parent   *AccessNode
	Children []*AccessNode
}

func (n *AccessNode) HasChild() bool {
	return len(n.Children) > 0
}

func (n *AccessNode) GetChild(name string, strict bool) *AccessNode {
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
			t, ok := s.m[host]
			if ok {
				result[host] = t.ToAccessRule()
			}
		}
	}
	return result
}

type AccessTree interface {
	GetRoot() *AccessNode
	ToAccessRule() []AccessRule
	Json() string
}

type accessTree struct {
	Root *AccessNode
}

func (t *accessTree) GetRoot() *AccessNode {
	return t.Root
}

func (t *accessTree) ToAccessRule() []AccessRule {
	return nil
}

func (t *accessTree) Json() string {
	b, _ := json.MarshalIndent(t.Root, "", "  ")
	return string(b)
}

func NewAccessTree(rules []AccessRule) AccessTree {
	root := &AccessNode{
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
