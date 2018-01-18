// Copyright 2018 Yunify Inc. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

package store

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAccessStore(t *testing.T) {
	accessStore := NewAccessStore()
	ip := "192.168.1.1"
	rules := []AccessRule{
		{Path: "/", Mode: AccessModeForbidden},
		{Path: "/clusters", Mode: AccessModeRead},
		{Path: "/clusters/cl-1/env/secret", Mode: AccessModeForbidden},
	}
	accessStore.Put(ip, rules)

	tree := accessStore.Get(ip)
	assert.NotNil(t, tree)

	ip2 := "192.168.1.2"
	accessStore.Puts(map[string][]AccessRule{
		ip2: rules,
	})
	rulesGet := accessStore.GetAccessRule([]string{ip})
	rules1 := rulesGet[ip]
	assert.Equal(t, rules, rules1)

	rulesGet2 := accessStore.GetAccessRule(nil)
	assert.Equal(t, rules, rulesGet2[ip])
	assert.Equal(t, rules, rulesGet2[ip2])

	accessStore.Delete(ip)
	rulesGet3 := accessStore.GetAccessRule([]string{})
	assert.Equal(t, 1, len(rulesGet3))
}

func TestAccessTree(t *testing.T) {
	rules := []AccessRule{
		{Path: "/", Mode: AccessModeForbidden},
		{Path: "/clusters", Mode: AccessModeRead},
		{Path: "/clusters/*/env", Mode: AccessModeForbidden},
		{Path: "/clusters/cl-1/env/secret", Mode: AccessModeRead},
	}
	tree := NewAccessTree(rules)
	jsonStr := tree.Json()
	jsonMap := map[string]interface{}{}
	err := json.Unmarshal([]byte(jsonStr), &jsonMap)
	assert.NoError(t, err)
	root := tree.GetRoot()
	assert.Equal(t, AccessModeForbidden, root.Mode)
	assert.Equal(t, AccessModeRead, root.GetChild("clusters", true).Mode)
	assert.Equal(t, AccessModeForbidden, root.GetChild("clusters", true).
		GetChild("cl-2", false).GetChild("env", true).Mode)
	assert.Equal(t, AccessModeRead, root.GetChild("clusters", true).
		GetChild("cl-1", false).GetChild("env", true).GetChild("secret", true).Mode)
}
