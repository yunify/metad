// Copyright 2018 Yunify Inc. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

package flatmap

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yunify/metad/log"
)

func init() {
	log.SetLevel("debug")
}

func TestFlatten(t *testing.T) {
	cases := []struct {
		Input  map[string]interface{}
		Output map[string]string
	}{
		{
			Input: map[string]interface{}{
				"foo": "bar",
				"bar": "baz",
			},
			Output: map[string]string{
				"/foo": "bar",
				"/bar": "baz",
			},
		},
		{
			Input: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": map[string]interface{}{
							"d": map[string]interface{}{
								"e": map[string]interface{}{
									"f": map[string]interface{}{
										"g": "val",
									},
								},
							},
						},
					},
				},
			},
			Output: map[string]string{
				"/a/b/c/d/e/f/g": "val",
			},
		},
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
		},
	}

	for _, tc := range cases {
		actual := Flatten(tc.Input)
		if !reflect.DeepEqual(actual, tc.Output) {
			t.Fatalf(
				"Input:\n\n%#v\n\nOutput:\n\n%#v\n\nExpected:\n\n%#v\n",
				tc.Input,
				actual,
				tc.Output)
		}
	}
}

func TestFlattenSlice(t *testing.T) {
	cases := []struct {
		Input  []interface{}
		Output map[string]string
	}{
		{
			Input: []interface{}{
				"foo", "bar",
			},
			Output: map[string]string{
				"/0": "foo",
				"/1": "bar",
			},
		},

		{
			Input: []interface{}{
				map[string]interface{}{
					"name":    "bar0",
					"port":    3000,
					"enabled": true,
				},
				map[string]interface{}{
					"name":    "bar1",
					"port":    3001,
					"enabled": false,
				},
			},
			Output: map[string]string{
				"/0/name":    "bar0",
				"/0/port":    "3000",
				"/0/enabled": "true",
				"/1/name":    "bar1",
				"/1/port":    "3001",
				"/1/enabled": "false",
			},
		},
	}

	for _, tc := range cases {
		actual := FlattenSlice(tc.Input)
		if !reflect.DeepEqual(actual, tc.Output) {
			t.Fatalf(
				"Input:\n\n%#v\n\nOutput:\n\n%#v\n\nExpected:\n\n%#v\n",
				tc.Input,
				actual,
				tc.Output)
		}
	}
}

func TestFlattenJSON(t *testing.T) {
	cases := []struct {
		Input  string
		Output map[string]string
	}{
		{
			Input: `
			{"hosts":
			    {"host1":
				{
          			"hostname": "hosts1",
          			"enable": true,
          			"primary_ip": "10.42.185.183",
          			"service_name": null,
          			"start_count": 1,
          			"labels": {"key":"value","key2":"value2"},
          			"ips": [
            				"10.42.185.183"
          			],
          			"int":9663676416,
				"float":1.1111111
        			}
        		    }
        		}
			`,
			Output: map[string]string{
				"/hosts/host1/hostname":     "hosts1",
				"/hosts/host1/enable":       "true",
				"/hosts/host1/primary_ip":   "10.42.185.183",
				"/hosts/host1/service_name": "",
				"/hosts/host1/start_count":  "1",
				"/hosts/host1/labels/key":   "value",
				"/hosts/host1/labels/key2":  "value2",
				"/hosts/host1/ips/0":        "10.42.185.183",
				"/hosts/host1/int":          "9663676416",
				"/hosts/host1/float":        "1.1111111",
			},
		},
	}

	for _, tc := range cases {
		var val interface{}
		err := json.Unmarshal([]byte(tc.Input), &val)
		assert.NoError(t, err)

		actual := Flatten(val)
		if !reflect.DeepEqual(actual, tc.Output) {
			t.Fatalf(
				"Input:\n\n%#v\n\nOutput:\n\n%#v\n\nExpected:\n\n%#v\n",
				tc.Input,
				actual,
				tc.Output)
		}
	}
}
