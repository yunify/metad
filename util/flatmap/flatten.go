// Copyright 2018 Yunify Inc. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

package flatmap

// Origin code is from
// https://github.com/hashicorp/terraform/blob/master/flatmap/flatten.go
// The different is this flatmap's path separator is '/', not '.', and with a different approach to process slice.

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"

	"github.com/yunify/metad/log"
)

// Flatten takes a structure and turns into a flat map[string]string.
//
// Within the "thing" parameter, only primitive values are allowed. Structs are
// not supported. Therefore, it can only be slices, maps, primitives, and
// any combination of those together.
//
//
//
// See the tests for examples of what inputs are turned into.
func Flatten(thing interface{}) map[string]string {
	result := make(map[string]string)
	switch t := thing.(type) {
	case map[string]interface{}:
		for k, raw := range t {
			flatten(result, k, reflect.ValueOf(raw))
		}
	case map[string]string:
		for k, raw := range t {
			flatten(result, k, reflect.ValueOf(raw))
		}
	case map[interface{}]interface{}:
		for k, raw := range t {
			flatten(result, fmt.Sprintf("%v", k), reflect.ValueOf(raw))
		}
	case []interface{}:
		for i, raw := range t {
			flatten(result, fmt.Sprintf("/%v", i), reflect.ValueOf(raw))
		}
	default:
		panic(errors.New(fmt.Sprintf("Unsupport type %v", t)))
	}

	return result
}

func FlattenSlice(thing []interface{}) map[string]string {
	result := make(map[string]string)

	for i, raw := range thing {
		flatten(result, fmt.Sprintf("/%v", i), reflect.ValueOf(raw))
	}

	return result
}

func flatten(result map[string]string, prefix string, v reflect.Value) {
	// skip empty key node.
	if prefix == "" {
		return
	}
	if v.Kind() == reflect.Interface {
		v = v.Elem()
	}
	if prefix[0] != '/' {
		prefix = "/" + prefix
	}
	if !v.IsValid() {
		result[prefix] = ""
	} else {
		switch v.Kind() {
		case reflect.Bool:
			if v.Bool() {
				result[prefix] = "true"
			} else {
				result[prefix] = "false"
			}
		case reflect.Int:
			result[prefix] = fmt.Sprintf("%v", v.Int())
		case reflect.Int64:
			result[prefix] = fmt.Sprintf("%v", v.Int())
		case reflect.Float64:
			s := strconv.FormatFloat(v.Float(), 'f', -1, 64)
			result[prefix] = s
		case reflect.Map:
			flattenMap(result, prefix, v)
		case reflect.Slice:
			flattenSlice(result, prefix, v)
		case reflect.String:
			result[prefix] = v.String()
		case reflect.Ptr:
			if v.IsNil() {
				result[prefix] = ""
			} else {
				log.Warning("Unknow type: %v", v.Type())
				result[prefix] = fmt.Sprintf("%v", v.Interface())
			}
		default:
			log.Warning("Unknow type: %v", v.Type())
			result[prefix] = fmt.Sprintf("%v", v.Interface())
		}
	}
}

func flattenMap(result map[string]string, prefix string, v reflect.Value) {
	for _, k := range v.MapKeys() {
		if k.Kind() == reflect.Interface {
			k = k.Elem()
		}
		var key string
		if k.Kind() != reflect.String {
			key = fmt.Sprintf("%v", k)
		} else {
			key = k.String()
		}

		flatten(result, fmt.Sprintf("%s/%s", prefix, key), v.MapIndex(k))
	}
}

func flattenSlice(result map[string]string, prefix string, v reflect.Value) {

	for i := 0; i < v.Len(); i++ {
		flatten(result, fmt.Sprintf("%s/%d", prefix, i), v.Index(i))
	}
}
