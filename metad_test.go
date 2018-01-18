// Copyright 2018 Yunify Inc. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yunify/metad/log"
	"github.com/yunify/metad/util"
)

var (
	testBackend = "local"
	sleepTime   = 100 * time.Millisecond
)

func init() {
	log.SetLevel("debug")
	rand.Seed(time.Now().UnixNano())
}

func TestMetad(t *testing.T) {
	metad := NewTestMetad()

	defer metad.Stop()

	dataJson := `
	{
		"nodes": {
	"1": {
	"ip": "192.168.1.1",
	"name": "node1"
	},
	"2": {
	"ip": "192.168.1.2",
	"name": "node2"
	},
	"3": {
	"ip": "192.168.1.3",
	"name": "node3"
	},
	"4": {
	"ip": "192.168.1.4",
	"name": "node4"
	},
	"5": {
	"ip": "192.168.1.5",
	"name": "node5"
	}
	}
	}
	`
	data := make(map[string]interface{})
	json.Unmarshal([]byte(dataJson), &data)

	req := httptest.NewRequest("GET", "/metrics", strings.NewReader(dataJson))
	w := httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	time.Sleep(sleepTime)

	req = httptest.NewRequest("GET", "/health", strings.NewReader(dataJson))
	w = httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	time.Sleep(sleepTime)

	req = httptest.NewRequest("PUT", "/v1/data/", strings.NewReader(dataJson))
	w = httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	time.Sleep(sleepTime)

	req = httptest.NewRequest("GET", "/v1/data/", nil)
	req.Header.Set("accept", "application/json")
	w = httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, data, parse(w))

	req = httptest.NewRequest("PUT", "/v1/data/nodes", strings.NewReader(`{"6":{"ip":"192.168.1.6","name":"node6"}}`))
	w = httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	time.Sleep(sleepTime)

	req = httptest.NewRequest("GET", "/v1/data/nodes", nil)
	req.Header.Set("accept", "application/json")
	w = httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "node6", util.GetMapValue(parse(w), "/6/name"))

	req = httptest.NewRequest("PUT", "/v1/data/nodes/6", strings.NewReader(`{"label":{"key1":"value1"}}`))
	w = httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	time.Sleep(sleepTime)

	req = httptest.NewRequest("GET", "/v1/data/nodes/6/label/key1", nil)
	req.Header.Set("accept", "application/json")
	w = httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "value1", parse(w))

	// test update by put
	req = httptest.NewRequest("PUT", "/v1/data/nodes/6", strings.NewReader(`{"label":{"key1":"new_value1"}}`))
	w = httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	time.Sleep(sleepTime)

	req = httptest.NewRequest("GET", "/v1/data/nodes/6/label/key1", nil)
	req.Header.Set("accept", "application/json")
	w = httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "new_value1", parse(w))

	// test replace by post
	req = httptest.NewRequest("POST", "/v1/data/nodes/6/label", strings.NewReader(`{"key3":"value3"}`))
	w = httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	time.Sleep(sleepTime)

	req = httptest.NewRequest("GET", "/v1/data/nodes/6/label", nil)
	req.Header.Set("accept", "application/json")
	w = httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	result := parse(w)
	v := parseVersion(w)
	assert.True(t, v > 0)
	m := result.(map[string]interface{})
	assert.Equal(t, "value3", m["key3"])
	//key1 has been replaced.
	assert.Equal(t, nil, m["key1"])

	//test mapping

	req = httptest.NewRequest("POST", "/v1/mapping", strings.NewReader(`{"192.168.1.1":{"node":"/nodes/1"}, "192.168.1.2":{"node":"/nodes/2"}, "192.168.1.3":{"node":"/nodes/3"}}`))
	w = httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	time.Sleep(sleepTime)

	//test self request /

	req = httptest.NewRequest("GET", "/self", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	req.Header.Set("accept", "application/json")
	w = httptest.NewRecorder()
	metad.router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "node1", util.GetMapValue(parse(w), "/node/name"))

	//test self request sub node
	req = httptest.NewRequest("GET", "/self/node/name", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	req.Header.Set("accept", "application/json")
	w = httptest.NewRecorder()
	metad.router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "node1", parse(w))

	// delete node1
	req = httptest.NewRequest("DELETE", "/v1/data/nodes/1", nil)
	w = httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	time.Sleep(sleepTime)

	// node1 self not found.
	req = httptest.NewRequest("GET", "/self/node/name", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	req.Header.Set("accept", "application/json")
	w = httptest.NewRecorder()
	metad.router.ServeHTTP(w, req)
	assert.Equal(t, 404, w.Code)

}

func TestMetadWatch(t *testing.T) {
	metad := NewTestMetad()

	defer metad.Stop()
	ip := "192.168.1.1"
	remoteAddr := ip + ":1234"

	dataJson := `
	{
		"nodes": {
	"1": {
	"ip": "192.168.1.1",
	"name": "node1"
	}
	}
	}
	`
	data := make(map[string]interface{})
	json.Unmarshal([]byte(dataJson), &data)

	req := httptest.NewRequest("PUT", "/v1/data/", strings.NewReader(dataJson))
	w := httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	ruleJson := `
	{"192.168.1.1":[{"path":"/","mode":1}]
	}
	`
	req = httptest.NewRequest("PUT", "/v1/rule/", strings.NewReader(ruleJson))
	w = httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	time.Sleep(sleepTime)
	versions := make(chan int, 1)
	go func() {
		req := httptest.NewRequest("GET", "/nodes/1/ip?wait=true", nil)
		req.Header.Set("accept", "application/json")
		req.RemoteAddr = remoteAddr
		w := httptest.NewRecorder()
		metad.router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
		version := parseVersion(w)
		versions <- version
	}()

	time.Sleep(sleepTime)

	req = httptest.NewRequest("PUT", "/v1/data/nodes/1/ip", strings.NewReader(`"192.168.2.1"`))
	w = httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	v := <-versions
	assert.True(t, v >= 0)

	// change again
	req = httptest.NewRequest("PUT", "/v1/data/nodes/1/ip", strings.NewReader(`"192.168.3.1"`))
	w = httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	//wait with prev_version should return immediately
	req = httptest.NewRequest("GET", fmt.Sprintf("/nodes/1/ip?wait=true&prev_version=%d", v), nil)
	req.Header.Set("accept", "application/json")
	req.RemoteAddr = remoteAddr
	w = httptest.NewRecorder()
	metad.router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	v2 := parseVersion(w)
	assert.True(t, v2 > v)
	assert.Equal(t, "192.168.3.1", parse(w))
}

func TestMetadWatchSelf(t *testing.T) {
	metad := NewTestMetad()

	defer metad.Stop()

	dataJson := `
	{
		"nodes": {
	"1": {
	"ip": "192.168.1.1",
	"name": "node1"
	}
	}
	}
	`
	data := make(map[string]interface{})
	json.Unmarshal([]byte(dataJson), &data)

	req := httptest.NewRequest("PUT", "/v1/data/", strings.NewReader(dataJson))
	w := httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	req = httptest.NewRequest("POST", "/v1/mapping", strings.NewReader(`{"192.168.1.1":{"node":"/nodes/1"}}`))
	w = httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	time.Sleep(sleepTime)

	//test self request
	req = httptest.NewRequest("GET", "/self", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	req.Header.Set("accept", "application/json")
	w = httptest.NewRecorder()
	metad.router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "node1", util.GetMapValue(parse(w), "/node/name"))

	versions := make(chan int, 1)
	go func() {
		req := httptest.NewRequest("GET", "/self?wait=true", nil)
		req.Header.Set("accept", "application/json")
		req.RemoteAddr = "192.168.1.1:1234"
		w := httptest.NewRecorder()
		metad.router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
		assert.Equal(t, "192.168.2.1", util.GetMapValue(parse(w), "/node/ip"))
		version := parseVersion(w)
		versions <- version
	}()

	time.Sleep(sleepTime)

	req = httptest.NewRequest("PUT", "/v1/data/nodes/1/ip", strings.NewReader(`"192.168.2.1"`))
	w = httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	v := <-versions
	assert.True(t, v >= 0)

	// change again
	req = httptest.NewRequest("PUT", "/v1/data/nodes/1/ip", strings.NewReader(`"192.168.3.1"`))
	w = httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	time.Sleep(sleepTime)

	//wait with prev_version should return immediately
	req = httptest.NewRequest("GET", fmt.Sprintf("/self?wait=true&prev_version=%d", v), nil)
	req.Header.Set("accept", "application/json")
	req.RemoteAddr = "192.168.1.1:1234"
	w = httptest.NewRecorder()
	metad.router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	v2 := parseVersion(w)
	assert.True(t, v2 > v)
	assert.Equal(t, "192.168.3.1", util.GetMapValue(parse(w), "/node/ip"))
}

func TestMetadMappingDelete(t *testing.T) {
	metad := NewTestMetad()

	defer metad.Stop()

	dataJson := `
	{
		"nodes": {
	"1": {
	"ip": "192.168.1.1",
	"name": "node1"
	}
	}
	}
	`
	data := make(map[string]interface{})
	json.Unmarshal([]byte(dataJson), &data)

	req := httptest.NewRequest("PUT", "/v1/data/", strings.NewReader(dataJson))
	w := httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	ip := "192.168.1.1"
	req = httptest.NewRequest("POST", "/v1/mapping", strings.NewReader(`{"192.168.1.1":{"node":"/nodes/1"}}`))
	w = httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	time.Sleep(sleepTime)

	getAndCheckMapping(metad, t, ip, true)

	req = httptest.NewRequest("DELETE", "/v1/mapping?subs=192.168.1.2,,", nil)
	w = httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	time.Sleep(sleepTime)

	getAndCheckMapping(metad, t, ip, true)

	req = httptest.NewRequest("DELETE", "/v1/mapping?subs=,,", nil)
	w = httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	time.Sleep(sleepTime)

	getAndCheckMapping(metad, t, ip, true)

	req = httptest.NewRequest("DELETE", "/v1/mapping?subs=192.168.1.1", nil)
	w = httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	time.Sleep(sleepTime)

	getAndCheckMapping(metad, t, ip, false)
}

func TestMetadAccessRule(t *testing.T) {
	metad := NewTestMetad()
	defer metad.Stop()

	data := map[string]interface{}{
		"clusters": map[string]interface{}{
			"cl-1": map[string]interface{}{
				"name": "cl-1",
				"env": map[string]interface{}{
					"username": "user1",
					"secret":   "123456",
				},
				"public_key": "public_key_val",
			},
			"cl-2": map[string]interface{}{
				"name": "cl-2",
				"env": map[string]interface{}{
					"username": "user2",
					"secret":   "1234567",
				},
				"public_key": "public_key_val2",
			},
		},
	}

	b, _ := json.Marshal(data)
	dataJson := string(b)
	req := httptest.NewRequest("PUT", "/v1/data/", strings.NewReader(dataJson))
	w := httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	mappingJson := `
	{"192.168.1.1":{"cluster":"/clusters/cl-1", "links":{"c2":"/clusters/cl-2"}},
	"192.168.1.2":{"cluster":"/clusters/cl-2"}}`
	req = httptest.NewRequest("POST", "/v1/mapping", strings.NewReader(mappingJson))
	w = httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	ruleJson := `
	{"192.168.1.1":[{"path":"/","mode":0}, {"path":"/clusters/*/env","mode":0},{"path":"/clusters/cl-1","mode":1}, {"path":"/clusters/cl-2","mode":1}, {"path":"/clusters/cl-2/env/secret","mode":0}],
	"192.168.1.2":[{"path":"/","mode":0}, {"path":"/clusters/*/env","mode":0},{"path":"/clusters/cl-2","mode":1}]
	}
	`
	req = httptest.NewRequest("POST", "/v1/rule", strings.NewReader(ruleJson))
	w = httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	time.Sleep(sleepTime)

	req = httptest.NewRequest("GET", "/v1/rule", nil)
	req.Header.Set("accept", "application/json")
	w = httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "/", util.GetMapValue(parse(w), "/192.168.1.1/0/path"))
	assert.Equal(t, "1", util.GetMapValue(parse(w), "/192.168.1.1/2/mode"))

	req = httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	req.Header.Set("accept", "application/json")
	w = httptest.NewRecorder()
	metad.router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "cl-1", util.GetMapValue(parse(w), "/self/cluster/name"))
	// node1 can access cl-2
	assert.Equal(t, "cl-2", util.GetMapValue(parse(w), "/clusters/cl-2/name"))
	assert.Equal(t, "user2", util.GetMapValue(parse(w), "/self/links/c2/env/username"))
	//can not access cl-2 env/secret
	assert.Equal(t, "", util.GetMapValue(parse(w), "/self/links/c2/env/secret"))

	req = httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.2:1234"
	req.Header.Set("accept", "application/json")
	w = httptest.NewRecorder()
	metad.router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "cl-2", util.GetMapValue(parse(w), "/self/cluster/name"))
	assert.Equal(t, "1234567", util.GetMapValue(parse(w), "/self/cluster/env/secret"))
	// node2 can not access cl-1
	assert.Equal(t, "", util.GetMapValue(parse(w), "/clusters/cl-1/name"))
}

func NewTestMetad() *Metad {
	group := fmt.Sprintf("/group%v", rand.Intn(10000))
	config := &Config{
		Backend: testBackend,
		Group:   group,
	}
	metad, err := New(config)
	if err != nil {
		panic(err)
	}

	metad.Init()
	return metad
}

func getAndCheckMapping(metad *Metad, t *testing.T, ip string, exist bool) {
	req := httptest.NewRequest("GET", "/v1/mapping", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	mappingJson := w.Body.String()
	mapping := make(map[string]interface{})
	err := json.Unmarshal([]byte(mappingJson), &mapping)
	if err != nil {
		t.Fatal("Unmarshal err:", mappingJson, err)
	}
	_, ok := mapping[ip]
	assert.True(t, ok == exist)
}

func parseVersion(w *httptest.ResponseRecorder) int {
	versionHeader := w.Header().Get("X-Metad-Version")
	var version int
	if versionHeader != "" {
		var err error
		version, err = strconv.Atoi(versionHeader)
		if err != nil {
			version = -1
		}
	}
	return version
}

func parse(w *httptest.ResponseRecorder) interface{} {
	requestID := w.Header().Get("X-Metad-RequestID")
	var result interface{}
	log.Debug("%s response %s", requestID, w.Body.String())
	err := json.Unmarshal(w.Body.Bytes(), &result)
	if err != nil {
		panic(fmt.Errorf("json_err: %s", err.Error()))
	}
	return result
}
