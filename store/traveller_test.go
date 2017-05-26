package store

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	//"encoding/json"
)

func TestNodeTraveller(t *testing.T) {
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
	traveller := s.Traveller(accessRules)
	nodeTraveller := traveller.(*nodeTraveller)
	fmt.Println(nodeTraveller.tree.Json())

	assert.True(t, traveller.Enter("/"))
	assert.True(t, traveller.Enter("clusters"))
	assert.True(t, traveller.Enter("cl-1"))
	assert.True(t, traveller.Enter("env"))
	//assert.True(t, EnterForbidden, traveller.Enter("env"))

	traveller = s.Traveller(accessRules)
	assert.True(t, traveller.Enter("/"))
	assert.True(t, traveller.Enter("clusters"))
	assert.True(t, traveller.Enter("cl-2"))
	assert.False(t, traveller.Enter("env"))

	traveller = s.Traveller(accessRules)
	assert.True(t, traveller.Enter("/"))
	assert.True(t, traveller.Enter("clusters"))
	assert.True(t, traveller.Enter("cl-2"))
	assert.True(t, traveller.Enter("public_key"))

	traveller = s.Traveller(accessRules)
	traveller.Enter("/")
	traveller.Enter("clusters")
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
