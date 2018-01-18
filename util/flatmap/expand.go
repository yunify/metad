// Copyright 2018 Yunify Inc. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

package flatmap

import (
	"path"
	"strings"
)

// Expand takes a map and a key (prefix) and expands that value into
// a more complex structure. This is the reverse of the Flatten operation
// but if origin map include slice, Expand(Flatten(map)) will lose the slice info, slice will treat as map with number key.
func Expand(m map[string]string, prefix string) map[string]interface{} {
	prefix = path.Clean(path.Join("/", prefix))
	if prefix[len(prefix)-1] != '/' {
		prefix = prefix + "/"
	}
	result := expandMap(m, prefix)
	return result
}

func expand(m map[string]string, prefix string) interface{} {
	if v, ok := m[prefix]; ok {
		return v
	}
	prefix = prefix + "/"
	for k, _ := range m {
		if strings.HasPrefix(k, prefix) {
			return expandMap(m, prefix)
		}
	}
	return expandMap(m, prefix)
}

func expandMap(m map[string]string, prefix string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, _ := range m {
		if prefix != "/" && !strings.HasPrefix(k, prefix) {
			continue
		}

		key := k[len(prefix):]
		idx := strings.Index(key, "/")
		if idx != -1 {
			key = key[:idx]
		}
		if _, ok := result[key]; ok {
			continue
		}

		// It contains a period, so it is a more complex structure
		result[key] = expand(m, k[:len(prefix)+len(key)])
	}

	return result
}
