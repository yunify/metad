package util

import (
	"path"
	"strings"
)

func TrimPathPrefix(metapath string, prefix string) string {
	prefix = path.Join("/", prefix)

	if prefix == "/" {
		return metapath
	}

	if prefix == metapath {
		return "/"
	}

	if prefix[len(prefix)-1] != '/' {
		prefix = prefix + "/"
	}
	metapath = path.Join("/", metapath)

	return path.Clean(path.Join("/", strings.TrimPrefix(metapath, prefix)))
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
	if strings.TrimSpace(metapath) == "" {
		return metapath
	}
	prefix = path.Join("/", prefix)
	return path.Clean(path.Join(prefix, metapath))
}
