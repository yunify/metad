package main

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/yunify/metad/log"
	"github.com/yunify/metad/util"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

var (
	testBackend = "local"
	sleepTime   = 100 * time.Millisecond
)

func init() {
	log.SetLevel("debug")
}

func TestMetad(t *testing.T) {
	config := &Config{
		Backend: testBackend,
	}
	metad, err := New(config)
	assert.NoError(t, err)

	metad.Init()

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

	req := httptest.NewRequest("PUT", "/v1/data/", strings.NewReader(dataJson))
	w := httptest.NewRecorder()
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
	config := &Config{
		Backend: testBackend,
	}
	metad, err := New(config)
	assert.NoError(t, err)

	metad.Init()

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

	time.Sleep(sleepTime)
	versions := make(chan int, 1)
	go func() {
		req := httptest.NewRequest("GET", "/nodes/1/ip?wait=true", nil)
		req.Header.Set("accept", "application/json")

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
	w = httptest.NewRecorder()
	metad.router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	v2 := parseVersion(w)
	assert.True(t, v2 > v)
	assert.Equal(t, "192.168.3.1", parse(w))
}

func TestMetadWatchSelf(t *testing.T) {
	config := &Config{
		Backend: testBackend,
	}
	metad, err := New(config)
	assert.NoError(t, err)

	metad.Init()

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
	config := &Config{
		Backend: testBackend,
	}
	metad, err := New(config)
	assert.NoError(t, err)

	metad.Init()

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
		log.Error("json_err: %s", err.Error())
	}
	return result
}
