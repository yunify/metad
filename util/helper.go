package util

import (
	"path"
	"strings"
)

func TrimPathPrefix(metapath string, prefix string) string {
	if prefix == "" || prefix == "/" {
		return metapath
	}
	return strings.TrimPrefix(metapath, prefix)
}

func TrimPathPrefixBatch(meta map[string]string, prefix string) map[string]string {
	newMeta := make(map[string]string)
	for k, v := range meta {
		newKey := TrimPathPrefix(k, prefix)
		newMeta[newKey] = v
	}
	return newMeta
}

func AppendPathPrefix(metapath string, prefix string) string {
	if prefix == "" || prefix == "/" {
		return metapath
	}
	return path.Clean(path.Join(prefix, metapath))
}
