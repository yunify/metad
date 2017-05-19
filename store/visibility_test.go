package store

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"fmt"
	"github.com/yunify/metad/util"
)

type TestCase struct {
	input             string
	expect_path       string
	expect_visibility VisibilityLevel
}

func TestGetVisibilityByLinkParameter(t *testing.T) {
	testCases := []TestCase{
		{
			input:             "/clusters/c1",
			expect_path:       "/clusters/c1",
			expect_visibility: VisibilityLevelNone,
		},
		{
			input:             "/clusters/c2?visibility=0",
			expect_path:       "/clusters/c2",
			expect_visibility: VisibilityLevelPublic,
		},
		{
			input:             "/clusters/c3?visibility=1",
			expect_path:       "/clusters/c3",
			expect_visibility: VisibilityLevelProtected,
		},
		{
			input:             "c4?visibility=2",
			expect_path:       "c4",
			expect_visibility: VisibilityLevelPrivate,
		},
		{
			input:             "/clusters/c1?visibility=100",
			expect_path:       "/clusters/c1",
			expect_visibility: VisibilityLevelNone,
		},
	}
	Assert(t, testCases, ParseVisibility, assert.Equal)
}

func Assert(t *testing.T, testCases []TestCase, function func(input string) (string, VisibilityLevel), assertFun func(t assert.TestingT, expected, actual interface{}, msgAndArgs ...interface{}) (bool)) {
	for idx, testCase := range testCases {
		path, visibility := function(testCase.input)
		assertFun(t, testCase.expect_visibility, visibility, fmt.Sprintf("TestCase [%v] func [%v] input [%v] expect [%v], but get [%v]", idx, util.GetFunctionName(function), testCase.input, testCase.expect_visibility, visibility))
		assertFun(t, testCase.expect_path, path, fmt.Sprintf("TestCase [%v] func [%v] input [%v] expect [%v], but get [%v]", idx, util.GetFunctionName(function), testCase.input, testCase.expect_path, path))
	}
}
