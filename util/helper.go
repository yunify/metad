// Copyright 2018 Yunify Inc. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

package util

import (
	"path"
	"strconv"
	"strings"

	"github.com/yunify/metad/util/flatmap"
)

func TrimPathPrefix(nodePath string, prefix string) string {
	prefix = path.Join("/", prefix)

	if prefix == "/" {
		return nodePath
	}

	if prefix == nodePath {
		return "/"
	}

	if prefix[len(prefix)-1] != '/' {
		prefix = prefix + "/"
	}
	nodePath = path.Join("/", nodePath)

	return path.Clean(path.Join("/", strings.TrimPrefix(nodePath, prefix)))
}

func TrimPathPrefixBatch(meta map[string]string, prefix string) map[string]string {
	newMeta := make(map[string]string)
	for k, v := range meta {
		newKey := TrimPathPrefix(k, prefix)
		newMeta[newKey] = v
	}
	return newMeta
}

func AppendPathPrefix(nodePath string, prefix string) string {
	if strings.TrimSpace(nodePath) == "" {
		return nodePath
	}
	prefix = path.Join("/", prefix)
	return path.Clean(path.Join(prefix, nodePath))
}

func GetMapValue(m interface{}, nodePath string) string {
	fm := flatmap.Flatten(m)
	v := fm[nodePath]
	return v
}

func ParseInt(value string, defaultValue int) int {
	if value == "" {
		return defaultValue
	}
	result, err := strconv.Atoi(value)
	if err != nil {
		result = defaultValue
	}
	return result
}
