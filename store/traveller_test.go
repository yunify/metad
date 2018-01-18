// Copyright 2018 Yunify Inc. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

package store

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTravellerStack(t *testing.T) {
	stack := &travellerStack{}

	assert.Nil(t, stack.Pop())

	one := &stackElement{node: nil, mode: AccessModeNil}
	two := &stackElement{node: nil, mode: AccessModeForbidden}
	three := &stackElement{node: nil, mode: AccessModeRead}
	stack.Push(one)
	stack.Push(two)
	stack.Push(three)

	assert.Equal(t, three, stack.Pop())
	assert.Equal(t, two, stack.Pop())
	assert.Equal(t, one, stack.Pop())

	assert.Nil(t, stack.Pop())
}

func TestTravellerEnter(t *testing.T) {
	s := New()
	data := map[string]interface{}{
		"clusters": map[string]interface{}{
			"cl-1": map[string]interface{}{
				"env": map[string]interface{}{
					"name":   "app1",
					"secret": "123456",
				},
				"public_key": "public_key_val",
			},
			"cl-2": map[string]interface{}{
				"env": map[string]interface{}{
					"name":   "app2",
					"secret": "1234567",
				},
				"public_key": "public_key_val2",
			},
		},
	}
	s.Put("/", data)

	accessRules := []AccessRule{
		{
			Path: "/",
			Mode: AccessModeRead,
		},
	}

	traveller := s.Traveller(NewAccessTree(accessRules))
	defer traveller.Close()
	assert.True(t, traveller.Enter("/clusters"))
	assert.True(t, traveller.Enter("/cl-1/env"))
	assert.True(t, traveller.Enter("name"))
	assert.Equal(t, "app1", traveller.GetValue())

	traveller.BackToRoot()
	assert.True(t, traveller.Enter("/clusters/cl-1/env/secret"))
	traveller.BackStep(2)
	assert.True(t, traveller.Enter("public_key"))
	assert.Equal(t, "public_key_val", traveller.GetValue())

	traveller.BackToRoot()
	assert.True(t, traveller.Enter("/"))

}

func TestTraveller(t *testing.T) {
	s := New()
	data := map[string]interface{}{
		"clusters": map[string]interface{}{
			"cl-1": map[string]interface{}{
				"env": map[string]interface{}{
					"name":   "app1",
					"secret": "123456",
				},
				"public_key": "public_key_val",
			},
			"cl-2": map[string]interface{}{
				"env": map[string]interface{}{
					"name":   "app2",
					"secret": "1234567",
				},
				"public_key": "public_key_val2",
			},
		},
	}
	s.Put("/", data)

	accessRules := []AccessRule{
		{
			Path: "/",
			Mode: AccessModeForbidden,
		},
		{
			Path: "/clusters",
			Mode: AccessModeRead,
		},
		{
			Path: "/clusters/*/env",
			Mode: AccessModeForbidden,
		},
		{
			Path: "/clusters/cl-1",
			Mode: AccessModeRead,
		},
	}
	traveller := s.Traveller(NewAccessTree(accessRules))
	defer traveller.Close()

	nodeTraveller := traveller.(*nodeTraveller)
	fmt.Println(nodeTraveller.access.Json())

	assert.True(t, traveller.Enter("/clusters/cl-1/env"))
	traveller.BackToRoot()

	assert.False(t, traveller.Enter("/clusters/cl-2/env"))
	assert.True(t, traveller.Enter("/clusters/cl-2/public_key"))

	traveller.BackToRoot()

	traveller.Enter("/clusters")
	//traveller.Enter("cl-2")
	v := traveller.GetValue()
	//j,_ := json.MarshalIndent(v, "", "  ")
	//fmt.Printf("%s", string(j))
	mVal, ok := v.(map[string]interface{})
	assert.True(t, ok)
	cl1 := mVal["cl-1"].(map[string]interface{})
	cl2 := mVal["cl-2"].(map[string]interface{})

	envM := cl1["env"].(map[string]interface{})
	assert.Equal(t, 2, len(envM))
	assert.Nil(t, cl2["env"])
}
