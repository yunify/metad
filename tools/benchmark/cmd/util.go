package cmd

import (
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
