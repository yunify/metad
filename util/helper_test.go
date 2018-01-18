// Copyright 2018 Yunify Inc. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrimPathPrefix(t *testing.T) {
	cases := []struct {
		Input  []string
		Output string
	}{
		{
			Input:  []string{"/path", ""},
			Output: "/path",
		},
		{
			Input:  []string{"/path", "/"},
			Output: "/path",
		},
		{
			Input:  []string{"/path/sub1", "/path"},
			Output: "/sub1",
		},
		{
			Input:  []string{"/path/sub1", "/path/sub"},
			Output: "/path/sub1",
		},
		{
			Input:  []string{"/path/sub1", "/path/sub1"},
			Output: "/",
		},
	}

	for _, tc := range cases {
		actual := TrimPathPrefix(tc.Input[0], tc.Input[1])

		if actual != tc.Output {
			t.Fatalf(
				"Input:\n\n%#v\n\nOutput:\n\n%#v\n\nExpected:\n\n%#v\n",
				tc.Input,
				actual,
				tc.Output)
		}
	}
}

func TestAppendPathPrefix(t *testing.T) {
	cases := []struct {
		Input  []string
		Output string
	}{
		{
			Input:  []string{"/path", ""},
			Output: "/path",
		},
		{
			Input:  []string{"/path", "/"},
			Output: "/path",
		},
		{
			Input:  []string{"sub1", "/path"},
			Output: "/path/sub1",
		},
		{
			Input:  []string{"sub1", "path"},
			Output: "/path/sub1",
		},
		{
			Input:  []string{"/sub1", "path"},
			Output: "/path/sub1",
		},
		{
			Input:  []string{"1", "/path/sub"},
			Output: "/path/sub/1",
		},
	}

	for _, tc := range cases {
		actual := AppendPathPrefix(tc.Input[0], tc.Input[1])

		if actual != tc.Output {
			t.Fatalf(
				"Input:\n\n%#v\n\nOutput:\n\n%#v\n\nExpected:\n\n%#v\n",
				tc.Input,
				actual,
				tc.Output)
		}
	}
}

func TestGetMapValue(t *testing.T) {
	m := map[string]interface{}{
		"nodes": map[string]interface{}{
			"1": map[string]interface{}{
				"name": "node1",
			},
		},
	}
	assert.Equal(t, GetMapValue(m, "/nodes/1/name"), "node1")
	assert.Equal(t, GetMapValue(m, "nodes/2"), "")
}
