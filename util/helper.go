package util

import (
	"github.com/yunify/metad/util/flatmap"
	"path"
	"strings"
	"reflect"
	"runtime"
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

func GetFunctionName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}
