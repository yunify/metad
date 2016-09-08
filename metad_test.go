package main

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/yunify/metad/log"
	"github.com/yunify/metad/util"
	"net/http/httptest"
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
	m := parse(w).(map[string]interface{})
	assert.Equal(t, "value3", m["key3"])
	//key1 has been replaced.
	assert.Equal(t, nil, m["key1"])

	//test mapping

	req = httptest.NewRequest("POST", "/v1/mapping", strings.NewReader(`{"192.168.1.1":{"node":"/nodes/1"}, "192.168.1.2":{"node":"/nodes/2"}, "192.168.1.3":{"node":"/nodes/3"}}`))
	w = httptest.NewRecorder()
	metad.manageRouter.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	time.Sleep(sleepTime)

	//test self request
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
	req.RemoteAddr = "192.168.1.1"
	req.Header.Set("accept", "application/json")
	w = httptest.NewRecorder()
	metad.router.ServeHTTP(w, req)
	assert.Equal(t, 404, w.Code)
}

func parse(w *httptest.ResponseRecorder) interface{} {
	var result interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	return result
}
