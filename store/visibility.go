package store

import (
	"context"
	"github.com/yunify/metad/log"
	"net/url"
	"strconv"
	"strings"
)

type VisibilityLevel int

const (
	VisibilityLevelNone VisibilityLevel = iota - 1
	VisibilityLevelPublic
	VisibilityLevelProtected
	VisibilityLevelPrivate
	end

	VisibilityKey = "visibility"
)

func WithVisibility(ctx context.Context, vlevel VisibilityLevel) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = context.WithValue(ctx, VisibilityKey, vlevel)
	return ctx
}

// ParseVisibility parse path?visibility=2 to path and VisibilityLevelPrivate
func ParseVisibility(link string) (string, VisibilityLevel) {
	var vLevel VisibilityLevel = VisibilityLevelNone
	if strings.ContainsRune(link, '?') {
		parts := strings.Split(link, "?")
		link = parts[0]
		query := parts[1]
		values, err := url.ParseQuery(query)
		if err != nil {
			log.Error("Parse url [%s] error [%s] ", link, err)
		} else {
			vLevelStr := values.Get(VisibilityKey)
			if vLevelStr != "" {
				vLevelInt, err := strconv.Atoi(vLevelStr)
				if err != nil {
					log.Error("Parse url [%s] query error [%s] ", link, err)
				} else {
					if vLevelInt < int(end) {
						vLevel = VisibilityLevel(vLevelInt)
					}
				}
			}
		}
	}
	return link, vLevel
}

func TrimVisibilityPrefix(name string) string {
	if len(name) <= 2 {
		return name
	}
	if name[0] == '_' {
		if name[1] == '@' || name[1] == '$' {
			return name[2:]
		}
	}
	return name
}
