// Copyright 2018 Yunify Inc. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

package flatmap

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExpand(t *testing.T) {
	cases := []struct {
		Map    map[string]string
		Prefix string
		Output map[string]interface{}
	}{

		{
			Map: map[string]string{
				"/foo/0": "one",
				"/foo/1": "two",
			},
			Output: map[string]interface{}{
				"foo": map[string]interface{}{
					"0": "one",
					"1": "two",
				},
			},
		},

		{
			Map: map[string]string{
				"/foo/0/name":    "bar",
				"/foo/0/port":    "3000",
				"/foo/0/enabled": "true",
			},
			Output: map[string]interface{}{
				"foo": map[string]interface{}{
					"0": map[string]interface{}{
						"name":    "bar",
						"port":    "3000",
						"enabled": "true",
					},
				},
			},
		},

		{
			Map: map[string]string{
				"/foo/0/name":    "bar",
				"/foo/0/ports/0": "1",
				"/foo/0/ports/1": "2",
			},
			Output: map[string]interface{}{
				"foo": map[string]interface{}{
					"0": map[string]interface{}{
						"name": "bar",
						"ports": map[string]interface{}{
							"0": "1",
							"1": "2",
						},
					},
				},
			},
		},
		{
			Map: map[string]string{
				"/prefix81/testkey/subkey1/subkey1sub1": "subkey1sub1_value",
				"/prefix81/testkey/subkey1/subkey1sub2": "subkey1sub2_value",
				"/prefix81/testkey/subkey1/subkey1sub3": "subkey1sub3_value",
			},
			Prefix: "/prefix81/testkey",
			Output: map[string]interface{}{
				"subkey1": map[string]interface{}{
					"subkey1sub1": "subkey1sub1_value",
					"subkey1sub2": "subkey1sub2_value",
					"subkey1sub3": "subkey1sub3_value",
				},
			},
		},
	}

	for _, tc := range cases {
		actual := Expand(tc.Map, tc.Prefix)
		assert.Equal(t, actual, tc.Output)
	}
}

func TestFlattenAndExpand(t *testing.T) {
	cases := []struct {
		Input        map[string]interface{}
		Output       map[string]string
		ExpandExpect map[string]interface{}
	}{
		{
			Input: map[string]interface{}{
				"foo": []string{
					"one",
					"two",
				},
			},
			Output: map[string]string{
				"/foo/0": "one",
				"/foo/1": "two",
			},
			ExpandExpect: map[string]interface{}{
				"foo": map[string]interface{}{
					"0": "one",
					"1": "two",
				},
			},
		},

		{
			Input: map[string]interface{}{
				"foo": []map[interface{}]interface{}{
					map[interface{}]interface{}{
						"name":    "bar",
						"port":    3000,
						"enabled": true,
					},
				},
			},
			Output: map[string]string{
				"/foo/0/name":    "bar",
				"/foo/0/port":    "3000",
				"/foo/0/enabled": "true",
			},
			ExpandExpect: map[string]interface{}{
				"foo": map[string]interface{}{
					"0": map[string]interface{}{
						"name":    "bar",
						"port":    "3000",
						"enabled": "true",
					},
				},
			},
		},

		{
			Input: map[string]interface{}{
				"foo": []map[interface{}]interface{}{
					map[interface{}]interface{}{
						"name": "bar",
						"ports": []string{
							"1",
							"2",
						},
					},
				},
			},
			Output: map[string]string{
				"/foo/0/name":    "bar",
				"/foo/0/ports/0": "1",
				"/foo/0/ports/1": "2",
			},
			ExpandExpect: map[string]interface{}{
				"foo": map[string]interface{}{
					"0": map[string]interface{}{
						"name": "bar",
						"ports": map[string]interface{}{
							"0": "1",
							"1": "2",
						},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		flatActual := Flatten(tc.Input)
		if !reflect.DeepEqual(flatActual, tc.Output) {
			t.Fatalf(
				"Flatten Input:\n\n%#v\n\nOutput:\n\n%#v\n\nExpected:\n\n%#v\n",
				tc.Input,
				flatActual,
				tc.Output)
		}
		expandActual := Expand(tc.Output, "")
		expandExpect := tc.ExpandExpect
		if expandExpect == nil {
			expandExpect = tc.Input
		}
		if !reflect.DeepEqual(expandActual, expandExpect) {
			t.Fatalf(
				"Expand Input:\n\n%#v\n\nOutput:\n\n%#v\n\nExpected:\n\n%#v\n",
				tc.Output,
				expandActual,
				expandExpect)
		}
	}
}
