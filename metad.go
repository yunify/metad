// Copyright 2018 Yunify Inc. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/golang/gddo/httputil"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	yaml "gopkg.in/yaml.v2"

	"github.com/yunify/metad/atomic"
	"github.com/yunify/metad/backends"
	"github.com/yunify/metad/log"
	"github.com/yunify/metad/metadata"
	"github.com/yunify/metad/store"
	"github.com/yunify/metad/util/flatmap"
)

const (
	ContentText     = 1
	ContentTypeText = "text/plain"
	ContentJSON     = 2
	ContentTypeJSON = "application/json"
	ContentYAML     = 3
	ContentTypeYAML = "application/yaml"
)

type HttpError struct {
	Status  int
	Message string
}

func NewHttpError(status int, Message string) *HttpError {
	return &HttpError{Status: status, Message: Message}
}

func NewServerError(error error) *HttpError {
	return &HttpError{Status: http.StatusInternalServerError, Message: error.Error()}
}

func (e HttpError) Error() string {
	return fmt.Sprintf("%s", e.Message)
}

type handleFunc func(ctx context.Context, req *http.Request) (int64, interface{}, *HttpError)
type manageFunc func(ctx context.Context, req *http.Request) (interface{}, *HttpError)

type Metad struct {
	config       *Config
	metadataRepo *metadata.MetadataRepo
	router       *mux.Router
	manageRouter *mux.Router
	requestIDGen atomic.AtomicLong
}

func New(config *Config) (*Metad, error) {

	backendsConfig := backends.Config{
		Backend:      config.Backend,
		BasicAuth:    config.BasicAuth,
		ClientCaKeys: config.ClientCaKeys,
		ClientCert:   config.ClientCert,
		ClientKey:    config.ClientKey,
		BackendNodes: config.BackendNodes,
		Password:     config.Password,
		Username:     config.Username,
		Prefix:       config.Prefix,
		Group:        config.Group,
	}

	storeClient, err := backends.New(backendsConfig)
	if err != nil {
		return nil, err
	}

	metadataRepo := metadata.New(storeClient)
	return &Metad{config: config, metadataRepo: metadataRepo, router: mux.NewRouter(), manageRouter: mux.NewRouter()}, nil
}

func (m *Metad) Init() {
	m.metadataRepo.StartSync()
	m.initRouter()
	m.initManageRouter()
}

func (m *Metad) initRouter() {
	m.router.HandleFunc("/favicon.ico", http.NotFound)

	m.router.HandleFunc("/self", m.handleWrapper(m.selfHandler)).
		Methods("GET", "HEAD")

	m.router.HandleFunc("/self/{nodePath:.*}", m.handleWrapper(m.selfHandler)).
		Methods("GET", "HEAD")

	m.router.HandleFunc("/{nodePath:.*}", m.handleWrapper(m.rootHandler)).
		Methods("GET", "HEAD")
}

func (m *Metad) initManageRouter() {
	m.manageRouter.HandleFunc("/favicon.ico", http.NotFound)
	m.manageRouter.Handle("/metrics", promhttp.Handler())
	m.manageRouter.HandleFunc("/health", func(arg1 http.ResponseWriter, arg2 *http.Request) {
		status := make(map[string]string)
		status["status"] = "up"
		result, _ := json.Marshal(status)
		arg1.Write(result)
	})

	v1 := m.manageRouter.PathPrefix("/v1").Subrouter()

	v1.HandleFunc("/mapping", m.manageWrapper(m.mappingGet)).Methods("GET")
	v1.HandleFunc("/mapping", m.manageWrapper(m.mappingUpdate)).Methods("POST", "PUT")
	v1.HandleFunc("/mapping", m.manageWrapper(m.mappingDelete)).Methods("DELETE")

	mapping := v1.PathPrefix("/mapping").Subrouter()
	//mapping.HandleFunc("", mappingGET).Methods("GET")
	mapping.HandleFunc("/{nodePath:.*}", m.manageWrapper(m.mappingGet)).Methods("GET")
	mapping.HandleFunc("/{nodePath:.*}", m.manageWrapper(m.mappingUpdate)).Methods("POST", "PUT")
	mapping.HandleFunc("/{nodePath:.*}", m.manageWrapper(m.mappingDelete)).Methods("DELETE")

	v1.HandleFunc("/data", m.manageWrapper(m.dataGet)).Methods("GET")
	v1.HandleFunc("/data", m.manageWrapper(m.dataUpdate)).Methods("POST", "PUT")
	v1.HandleFunc("/data", m.manageWrapper(m.dataDelete)).Methods("DELETE")

	data := v1.PathPrefix("/data").Subrouter()
	//mapping.HandleFunc("", mappingGET).Methods("GET")
	data.HandleFunc("/{nodePath:.*}", m.manageWrapper(m.dataGet)).Methods("GET")
	data.HandleFunc("/{nodePath:.*}", m.manageWrapper(m.dataUpdate)).Methods("POST", "PUT")
	data.HandleFunc("/{nodePath:.*}", m.manageWrapper(m.dataDelete)).Methods("DELETE")

	v1.HandleFunc("/rule", m.manageWrapper(m.accessRuleGet)).Methods("GET")
	v1.HandleFunc("/rule", m.manageWrapper(m.accessRuleUpdate)).Methods("POST", "PUT")
	v1.HandleFunc("/rule", m.manageWrapper(m.accessRuleDelete)).Methods("DELETE")

	rule := v1.PathPrefix("/rule").Subrouter()
	rule.HandleFunc("/", m.manageWrapper(m.accessRuleGet)).Methods("GET")
	rule.HandleFunc("/", m.manageWrapper(m.accessRuleUpdate)).Methods("POST", "PUT")
	rule.HandleFunc("/", m.manageWrapper(m.accessRuleDelete)).Methods("DELETE")
}

func (m *Metad) Serve() {
	m.watchSignals()
	m.watchManage()

	log.Info("Listening on %s", m.config.Listen)
	log.Fatal("%v", http.ListenAndServe(m.config.Listen, m.router))
}

func (m *Metad) Stop() {
	m.metadataRepo.StopSync()
}

func (m *Metad) watchSignals() {
	notifier := make(chan os.Signal, 1)
	signal.Notify(notifier, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-notifier
		log.Info("Received stop signal")
		signal.Stop(notifier)
		m.Stop()
		pid := syscall.Getpid()
		// exit directly if it is the "init" process, since the kernel will not help to kill pid 1.
		if pid == 1 {
			os.Exit(0)
		}
		syscall.Kill(pid, sig.(syscall.Signal))
	}()
}

func (m *Metad) watchManage() {
	log.Info("Listening for Manage on %s", m.config.ListenManage)
	go http.ListenAndServe(m.config.ListenManage, m.manageRouter)
}

func (m *Metad) dataGet(ctx context.Context, req *http.Request) (interface{}, *HttpError) {
	vars := mux.Vars(req)
	nodePath := vars["nodePath"]
	if nodePath == "" {
		nodePath = "/"
	}
	val := m.metadataRepo.GetData(nodePath)
	if val == nil {
		return nil, NewHttpError(http.StatusNotFound, "Not found")
	} else {
		return val, nil
	}
}

func (m *Metad) dataUpdate(ctx context.Context, req *http.Request) (interface{}, *HttpError) {
	vars := mux.Vars(req)
	nodePath := vars["nodePath"]
	if nodePath == "" {
		nodePath = "/"
	}
	decoder := json.NewDecoder(req.Body)
	var data interface{}
	err := decoder.Decode(&data)
	if err != nil {
		return nil, NewHttpError(http.StatusBadRequest, fmt.Sprintf("invalid json format, error:%s", err.Error()))
	} else {
		// POST means replace old value
		// PUT means merge to old value
		replace := "POST" == strings.ToUpper(req.Method)
		err = m.metadataRepo.PutData(nodePath, data, replace)
		if err != nil {
			if log.IsDebugEnable() {
				log.Debug("dataUpdate  nodePath:%s, data:%v, error:%s", nodePath, data, err.Error())
			}
			return nil, NewServerError(err)
		} else {
			return nil, nil
		}
	}
}

func (m *Metad) dataDelete(ctx context.Context, req *http.Request) (interface{}, *HttpError) {
	vars := mux.Vars(req)
	nodePath := vars["nodePath"]
	if nodePath == "" {
		nodePath = "/"
	}
	subsParam := req.FormValue("subs")
	var subs []string
	if subsParam != "" {
		subs = strings.Split(subsParam, ",")
	}
	err := m.metadataRepo.DeleteData(nodePath, subs...)
	if err != nil {
		return nil, NewServerError(err)
	} else {
		return nil, nil
	}
}

func (m *Metad) mappingGet(ctx context.Context, req *http.Request) (interface{}, *HttpError) {
	vars := mux.Vars(req)
	nodePath := vars["nodePath"]
	if nodePath == "" {
		nodePath = "/"
	}
	val := m.metadataRepo.GetMapping(nodePath)
	if val == nil {
		return nil, NewHttpError(http.StatusNotFound, "Not found")
	} else {
		return val, nil
	}
}

func (m *Metad) mappingUpdate(ctx context.Context, req *http.Request) (interface{}, *HttpError) {
	vars := mux.Vars(req)
	nodePath := vars["nodePath"]
	if nodePath == "" {
		nodePath = "/"
	}
	buf, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, NewHttpError(http.StatusBadRequest, fmt.Sprintf("read request error:%s", err.Error()))
	}
	log.Info("%s\tBODY\t%s", ctx.Value("requestID"), string(buf))
	decoder := json.NewDecoder(bytes.NewReader(buf))
	var data interface{}
	err = decoder.Decode(&data)
	if err != nil {
		return nil, NewHttpError(http.StatusBadRequest, fmt.Sprintf("invalid json format, error:%s", err.Error()))
	} else {
		// POST means replace old value
		// PUT means merge to old value
		replace := "POST" == strings.ToUpper(req.Method)
		err = m.metadataRepo.PutMapping(nodePath, data, replace)
		if err != nil {
			if log.IsDebugEnable() {
				log.Debug("mappingUpdate  nodePath:%s, data:%v, error:%s", nodePath, data, err.Error())
			}
			return nil, NewServerError(err)
		} else {
			return nil, nil
		}
	}
}

func (m *Metad) mappingDelete(ctx context.Context, req *http.Request) (interface{}, *HttpError) {
	vars := mux.Vars(req)
	nodePath := vars["nodePath"]
	if nodePath == "" {
		nodePath = "/"
	}
	subsParam := req.FormValue("subs")
	var subs []string
	if subsParam != "" {
		subs = strings.Split(subsParam, ",")
	}
	err := m.metadataRepo.DeleteMapping(nodePath, subs...)
	if err != nil {
		return nil, NewServerError(err)
	} else {
		return nil, nil
	}
}

func (m *Metad) accessRuleGet(ctx context.Context, req *http.Request) (interface{}, *HttpError) {
	hostsStr := req.FormValue("hosts")
	var hosts []string
	if hostsStr != "" {
		hosts = strings.Split(hostsStr, ",")
	}
	val := m.metadataRepo.GetAccessRule(hosts)
	return val, nil
}

func (m *Metad) accessRuleUpdate(ctx context.Context, req *http.Request) (interface{}, *HttpError) {
	decoder := json.NewDecoder(req.Body)
	var data map[string][]store.AccessRule
	err := decoder.Decode(&data)
	if err != nil {
		return nil, NewHttpError(http.StatusBadRequest, fmt.Sprintf("invalid json format, error:%s", err.Error()))
	} else {
		err = m.metadataRepo.PutAccessRule(data)
		if err != nil {
			if log.IsDebugEnable() {
				log.Debug("accessRuleUpdate data:%v, error:%s", data, err.Error())
			}
			return nil, NewServerError(err)
		} else {
			return nil, nil
		}
	}
}

func (m *Metad) accessRuleDelete(ctx context.Context, req *http.Request) (interface{}, *HttpError) {
	hostsStr := req.FormValue("hosts")
	var hosts []string
	if hostsStr != "" {
		hosts = strings.Split(hostsStr, ",")
	}
	err := m.metadataRepo.DeleteAccessRule(hosts)
	return nil, NewServerError(err)
}

func contentType(req *http.Request) int {
	str := httputil.NegotiateContentType(req, []string{
		"text/plain",
		"application/json",
		"application/yaml",
		"application/x-yaml",
		"text/yaml",
		"text/x-yaml",
	}, "text/plain")

	if strings.Contains(str, "json") {
		return ContentJSON
	} else if strings.Contains(str, "yaml") {
		return ContentYAML
	} else {
		return ContentText
	}
}

func (m *Metad) rootHandler(ctx context.Context, req *http.Request) (currentVersion int64, result interface{}, httpErr *HttpError) {
	clientIP := m.requestIP(req)
	vars := mux.Vars(req)
	nodePath := vars["nodePath"]
	if nodePath == "" {
		nodePath = "/"
	}
	wait := strings.ToLower(req.FormValue("wait")) == "true"
	if wait {
		prevVersionStr := req.FormValue("prev_version")
		var prevVersion int64
		if prevVersionStr != "" {
			var err error
			prevVersion, err = strconv.ParseInt(prevVersionStr, 10, 64)
			if err != nil {
				prevVersion = -1
			}
		}
		if prevVersion > 0 && prevVersion != m.metadataRepo.DataVersion() {
			currentVersion, result = m.metadataRepo.Root(clientIP, nodePath)
		} else {
			m.metadataRepo.Watch(ctx, clientIP, nodePath)
			// directly return new result to client ,not change, for keep same as request with prev_version
			currentVersion, result = m.metadataRepo.Root(clientIP, nodePath)
		}
	} else {
		currentVersion, result = m.metadataRepo.Root(clientIP, nodePath)
	}
	if result == nil {
		httpErr = NewHttpError(http.StatusNotFound, "Not found")
	}
	return
}

func (m *Metad) selfHandler(ctx context.Context, req *http.Request) (currentVersion int64, result interface{}, httpErr *HttpError) {
	clientIP := m.requestIP(req)
	vars := mux.Vars(req)
	nodePath := vars["nodePath"]
	if nodePath == "" {
		nodePath = "/"
	}
	wait := strings.ToLower(req.FormValue("wait")) == "true"
	// TODO this version may be not match the data, get version first, may be cause client repeat get data, but not lost change, so it work for now.
	currentVersion = m.metadataRepo.DataVersion()
	if wait {
		prevVersionStr := req.FormValue("prev_version")
		var prevVersion int64
		if prevVersionStr != "" {
			var err error
			prevVersion, err = strconv.ParseInt(prevVersionStr, 10, 64)
			if err != nil {
				prevVersion = -1
			}
		}
		// if prevVersion < currentVersion, client lost change, so return immediately.
		// if prevVersion > currentVersion, may be metad reboot and recount version, so return immediately, let client use new version.
		if prevVersion > 0 && prevVersion != currentVersion {
			result = m.metadataRepo.Self(clientIP, nodePath)
		} else {
			m.metadataRepo.WatchSelf(ctx, clientIP, nodePath)
			// directly return new result to client ,not change, for pre_version.
			result = m.metadataRepo.Self(clientIP, nodePath)
		}
	} else {
		result = m.metadataRepo.Self(clientIP, nodePath)
	}
	if result == nil {
		httpErr = NewHttpError(http.StatusNotFound, "Not found")
	}
	return
}

func respondError(w http.ResponseWriter, req *http.Request, msg string, statusCode int) {
	obj := make(map[string]interface{})
	obj["message"] = msg
	obj["type"] = "ERROR"
	obj["code"] = statusCode

	switch contentType(req) {
	case ContentText:
		http.Error(w, msg, statusCode)
	case ContentJSON:
		bytes, err := json.Marshal(obj)
		if err == nil {
			http.Error(w, string(bytes), statusCode)
		} else {
			http.Error(w, "{\"type\": \"error\", \"message\": \"JSON marshal error\"}", http.StatusInternalServerError)
		}
	case ContentYAML:
		bytes, err := yaml.Marshal(obj)
		if err == nil {
			http.Error(w, string(bytes), statusCode)
		} else {
			http.Error(w, "type: \"error\"\nmessage: \"JSON marshal error\"", http.StatusInternalServerError)
		}
	}
}

func respondSuccessDefault(w http.ResponseWriter, req *http.Request) {
	obj := make(map[string]interface{})
	obj["type"] = "OK"
	obj["code"] = 200
	switch contentType(req) {
	case ContentText:
		respondText(w, req, "OK")
	case ContentJSON:
		respondJSON(w, req, obj)
	case ContentYAML:
		respondYAML(w, req, obj)
	}
}

func respondSuccess(w http.ResponseWriter, req *http.Request, val interface{}) int {
	switch contentType(req) {
	case ContentText:
		return respondText(w, req, val)
	case ContentJSON:
		return respondJSON(w, req, val)
	case ContentYAML:
		return respondYAML(w, req, val)
	}
	return 0
}

func respondText(w http.ResponseWriter, req *http.Request, val interface{}) int {
	w.Header().Set("Content-Type", ContentTypeText)
	if val == nil {
		fmt.Fprint(w, "")
		return 0
	}
	var buffer bytes.Buffer
	switch v := val.(type) {
	case string:
		buffer.WriteString(v)
	case map[string]interface{}:
		fm := flatmap.Flatten(v)
		var keys []string
		for k := range fm {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			buffer.WriteString(k)
			buffer.WriteString("\t")
			buffer.WriteString(fm[k])
			buffer.WriteString("\n")
		}
	default:
		log.Error("Value is of a type I don't know how to handle: %v", val)
	}
	w.Write(buffer.Bytes())
	return buffer.Len()
}

func respondJSON(w http.ResponseWriter, req *http.Request, val interface{}) int {
	w.Header().Set("Content-Type", ContentTypeJSON)
	if val == nil {
		val = make(map[string]string)
	}
	prettyParam := req.FormValue("pretty")
	pretty := prettyParam != "" && prettyParam != "false"
	var bytes []byte
	var err error
	if pretty {
		bytes, err = json.MarshalIndent(val, "", "  ")
	} else {
		bytes, err = json.Marshal(val)
	}

	if err == nil {
		w.Write(bytes)
	} else {
		respondError(w, req, "Error serializing to JSON: "+err.Error(), http.StatusInternalServerError)
	}
	return len(bytes)
}

func respondYAML(w http.ResponseWriter, req *http.Request, val interface{}) int {
	w.Header().Set("Content-Type", ContentTypeYAML)
	bytes, err := yaml.Marshal(val)
	if err == nil {
		w.Write(bytes)
	} else {
		respondError(w, req, "Error serializing to YAML: "+err.Error(), http.StatusInternalServerError)
	}
	return len(bytes)
}

func (m *Metad) requestIP(req *http.Request) string {
	if m.config.EnableXff {
		clientIp := req.Header.Get("X-Forwarded-For")
		if len(clientIp) > 0 {
			return clientIp
		}
	}

	clientIp, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		log.Error("Get RequestIP error: %s", err.Error())
	}
	return clientIp
}

func (m *Metad) handleWrapper(handler handleFunc) func(w http.ResponseWriter, req *http.Request) {

	return func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		requestID := m.generateRequestID()

		ctx := context.WithValue(req.Context(), "requestID", requestID)
		cancelCtx, cancelFun := context.WithCancel(ctx)
		if x, ok := w.(http.CloseNotifier); ok {
			closeNotify := x.CloseNotify()
			go func() {
				select {
				case <-closeNotify:
					cancelFun()
				}
			}()
		} else {
			defer cancelFun()
		}
		version, result, err := handler(cancelCtx, req)

		w.Header().Add("X-Metad-RequestID", requestID)
		w.Header().Add("X-Metad-Version", fmt.Sprintf("%d", version))
		elapsed := time.Since(start)
		status := 200
		var len int
		if err != nil {
			status = err.Status
			respondError(w, req, err.Message, status)
			m.errorLog(requestID, req, status, err.Message)
		} else {
			if result == nil {
				respondSuccessDefault(w, req)
			} else {
				len = respondSuccess(w, req, result)
				if log.IsDebugEnable() {
					log.Debug("%s\tRESP\t%v", requestID, result)
				}
			}
		}
		m.requestLog(requestID, version, req, status, elapsed, len)
	}
}

func (m *Metad) manageWrapper(manager manageFunc) func(w http.ResponseWriter, req *http.Request) {

	return func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		requestID := m.generateRequestID()
		ctx := context.WithValue(req.Context(), "requestID", requestID)
		result, err := manager(ctx, req)
		version := m.metadataRepo.DataVersion()

		w.Header().Add("X-Metad-RequestID", requestID)
		w.Header().Add("X-Metad-Version", fmt.Sprintf("%d", version))
		elapsed := time.Since(start)
		status := 200
		var len int
		if err != nil {
			status = err.Status
			respondError(w, req, err.Message, status)
			m.errorLog(requestID, req, status, err.Message)
		} else {
			if result == nil {
				respondSuccessDefault(w, req)
			} else {
				len = respondSuccess(w, req, result)
				if log.IsDebugEnable() {
					log.Debug("%s\tRESP\t%v", requestID, result)
				}
			}
		}
		m.requestLog(requestID, version, req, status, elapsed, len)
	}
}

func (m *Metad) generateRequestID() string {
	return fmt.Sprintf("REQ-%d", m.requestIDGen.IncrementAndGet())
}

func (m *Metad) requestLog(requestID string, version int64, req *http.Request, status int, elapsed time.Duration, len int) {
	log.Info("%s\t%d\t%s\t%s\t%s\t%v\t%v\t%v\t%v", requestID, version, req.Method, m.requestIP(req), req.URL.RequestURI(), req.ContentLength, status, int64(elapsed.Seconds()*1000), len)
}

func (m *Metad) errorLog(requestID string, req *http.Request, status int, msg string) {
	if status == 500 {
		log.Error("ERR\t%s\t%s\t%s\t%s\t%v\t%v\t%s", requestID, req.Method, m.requestIP(req), req.RequestURI, req.ContentLength, status, msg)
	} else {
		log.Warning("ERR\t%s\t%s\t%s\t%s\t%v\t%v\t%s", requestID, req.Method, m.requestIP(req), req.RequestURI, req.ContentLength, status, msg)
	}
}
