// Copyright 2018 Yunify Inc. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

package cmd

import (
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

const defaultTimeout = 1000 * time.Millisecond

var (
	totalRequest = 0
)

func makeHttpClients(totalClients uint) []*http.Client {
	clients := make([]*http.Client, totalClients)
	for i := range clients {
		transport := &http.Transport{}
		transport.MaxIdleConnsPerHost = 10000
		clients[i] = &http.Client{Transport: transport, Timeout: defaultTimeout}
	}
	return clients
}

func makeMetaRequest(path string) *http.Request {
	req, _ := http.NewRequest("GET", strings.Join([]string{getEndpoint(), path}, ""), nil)
	req.Header.Set("Accept", "application/json")
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	totalRequest = totalRequest + 1
	return req
}

func makeManageRequest(method, api string, body io.Reader) *http.Request {
	req, _ := http.NewRequest(method, strings.Join([]string{manageEndpoint, api}, ""), body)
	req.Header.Set("Accept", "application/json")
	return req
}

func fillMetadata(data map[string]interface{}, mapping map[string]interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	jsonMapping, err := json.Marshal(mapping)
	if err != nil {
		panic(err)
	}

	client := makeHttpClients(1)[0]

	dataReq := makeManageRequest("POST", "/v1/data", strings.NewReader(string(jsonData)))

	resp, err := client.Do(dataReq)

	if err != nil {
		panic(err)
	}
	if resp.StatusCode != 200 {
		panic("fill metad data error")
	}
	if resp != nil {
		resp.Body.Close()
	}

	mappingReq := makeManageRequest("POST", "/v1/mapping", strings.NewReader(string(jsonMapping)))

	resp, err = client.Do(mappingReq)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode != 200 {
		panic("fill metad mapping error")
	}
	if resp != nil {
		resp.Body.Close()
	}
	jsonRule := `
		{"127.0.0.1":[{"path":"/", "mode":1}]}
	`
	ruleReq := makeManageRequest("POST", "/v1/rule", strings.NewReader(string(jsonRule)))

	resp, err = client.Do(ruleReq)
	if err != nil {
		panic(err)
	}
	//ignore 404 for old metad version not support /v1/rule api.
	if resp.StatusCode != 200 && resp.StatusCode != 404 {
		panic("fill metad rule error")
	}
	if resp != nil {
		resp.Body.Close()
	}
}

func getEndpoint() string {
	if len(endpoints) == 1 {
		return endpoints[0]
	}
	return endpoints[totalRequest%len(endpoints)]
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

var src = rand.NewSource(time.Now().UnixNano())

//https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-golang
func RandomString(n int) string {
	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}

func clientDo(client *http.Client, requests <-chan *http.Request) {
	defer wg.Done()

	for req := range requests {
		st := time.Now()
		resp, err := client.Do(req)
		var errStr string
		if err != nil {
			errStr = err.Error()
		}
		results <- result{errStr: errStr, duration: time.Since(st), happened: time.Now()}
		if resp != nil {
			resp.Body.Close()
		}
		bar.Increment()
	}
}
