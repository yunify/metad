package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/golang/gddo/httputil"
	"github.com/gorilla/mux"
	"github.com/yunify/metad/backends"
	"github.com/yunify/metad/log"
	"github.com/yunify/metad/metadata"
	"github.com/yunify/metad/util/flatmap"
	yaml "gopkg.in/yaml.v2"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"
)

const (
	ContentText = 1
	ContentJSON = 2
	ContentYAML = 3
)

var (
	router = mux.NewRouter()

	resyncChan = make(chan chan error)
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

type handlerFunc func(req *http.Request) (interface{}, *HttpError)

func main() {

	defer func() {
		if r := recover(); r != nil {
			log.Error("Main Recover: %v, try restart.", r)
			time.Sleep(time.Duration(1000) * time.Millisecond)
			main()
		}
	}()

	flag.Parse()

	if printVersion {
		fmt.Printf("%s\n", VERSION)
		os.Exit(0)
	}

	if err := initConfig(); err != nil {
		log.Fatal(err.Error())
		os.Exit(-1)
	}

	log.Info("Starting metad %s", VERSION)
	var err error
	storeClient, err = backends.New(backendsConfig)
	if err != nil {
		log.Fatal(err.Error())
		os.Exit(-1)
	}

	metadataRepo = metadata.New(config.OnlySelf, storeClient)

	metadataRepo.StartSync()

	watchSignals()
	watchManage()

	router.HandleFunc("/favicon.ico", http.NotFound)

	router.HandleFunc("/self", handlerWrapper(selfHandler)).
		Methods("GET", "HEAD")

	router.HandleFunc("/self/{nodePath:.*}", handlerWrapper(selfHandler)).
		Methods("GET", "HEAD")

	router.HandleFunc("/{nodePath:.*}", handlerWrapper(rootHandler)).
		Methods("GET", "HEAD")

	log.Info("Listening on %s", config.Listen)
	log.Fatal("%v", http.ListenAndServe(config.Listen, router))
}

func watchSignals() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP)

	go func() {
		for range c {
			log.Info("Received HUP signal")
			resyncChan <- nil
		}
	}()

	go func() {
		for resp := range resyncChan {
			err := resync()
			if resp != nil {
				resp <- err
			}
		}
	}()

	notifier := make(chan os.Signal, 1)
	signal.Notify(notifier, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-notifier
		log.Info("Received stop signal")
		signal.Stop(notifier)
		pid := syscall.Getpid()
		// exit directly if it is the "init" process, since the kernel will not help to kill pid 1.
		if pid == 1 {
			os.Exit(0)
		}
		syscall.Kill(pid, sig.(syscall.Signal))
	}()
}

func resync() error {
	metadataRepo.ReSync()
	return nil
}

func watchManage() {
	manageRouter := mux.NewRouter()
	manageRouter.HandleFunc("/favicon.ico", http.NotFound)

	v1 := manageRouter.PathPrefix("/v1").Subrouter()
	v1.HandleFunc("/resync", handlerWrapper(httpResync)).Methods("POST")

	v1.HandleFunc("/mapping", handlerWrapper(mappingGet)).Methods("GET")
	v1.HandleFunc("/mapping", handlerWrapper(mappingUpdate)).Methods("POST", "PUT")
	v1.HandleFunc("/mapping", handlerWrapper(mappingDelete)).Methods("DELETE")

	mapping := v1.PathPrefix("/mapping").Subrouter()
	//mapping.HandleFunc("", mappingGET).Methods("GET")
	mapping.HandleFunc("/{nodePath:.*}", handlerWrapper(mappingGet)).Methods("GET")
	mapping.HandleFunc("/{nodePath:.*}", handlerWrapper(mappingUpdate)).Methods("POST", "PUT")
	mapping.HandleFunc("/{nodePath:.*}", handlerWrapper(mappingDelete)).Methods("DELETE")

	v1.HandleFunc("/data", handlerWrapper(dataGet)).Methods("GET")
	v1.HandleFunc("/data", handlerWrapper(dataUpdate)).Methods("POST", "PUT")
	v1.HandleFunc("/data", handlerWrapper(dataDelete)).Methods("DELETE")

	data := v1.PathPrefix("/data").Subrouter()
	//mapping.HandleFunc("", mappingGET).Methods("GET")
	data.HandleFunc("/{nodePath:.*}", handlerWrapper(dataGet)).Methods("GET")
	data.HandleFunc("/{nodePath:.*}", handlerWrapper(dataUpdate)).Methods("POST", "PUT")
	data.HandleFunc("/{nodePath:.*}", handlerWrapper(dataDelete)).Methods("DELETE")

	log.Info("Listening for Manage on %s", config.ListenManage)
	go http.ListenAndServe(config.ListenManage, manageRouter)
}

func httpResync(req *http.Request) (interface{}, *HttpError) {
	respChan := make(chan error)
	resyncChan <- respChan
	err := <-respChan
	if err == nil {
		return nil, nil
	} else {
		return nil, NewServerError(err)
	}
}

func dataGet(req *http.Request) (interface{}, *HttpError) {
	vars := mux.Vars(req)
	nodePath := vars["nodePath"]
	if nodePath == "" {
		nodePath = "/"
	}
	val := metadataRepo.GetData(nodePath)
	if val == nil {
		return nil, NewHttpError(http.StatusNotFound, "Not found")
	} else {
		return val, nil
	}
}

func dataUpdate(req *http.Request) (interface{}, *HttpError) {
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
		err = metadataRepo.PutData(nodePath, data, replace)
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

func dataDelete(req *http.Request) (interface{}, *HttpError) {
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
	err := metadataRepo.DeleteData(nodePath, subs...)
	if err != nil {
		return nil, NewServerError(err)
	} else {
		return nil, nil
	}
}

func mappingGet(req *http.Request) (interface{}, *HttpError) {
	vars := mux.Vars(req)
	nodePath := vars["nodePath"]
	if nodePath == "" {
		nodePath = "/"
	}
	val := metadataRepo.GetMapping(nodePath)
	if val == nil {
		return nil, NewHttpError(http.StatusNotFound, "Not found")
	} else {
		return val, nil
	}
}

func mappingUpdate(req *http.Request) (interface{}, *HttpError) {
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
		err = metadataRepo.PutMapping(nodePath, data, replace)
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

func mappingDelete(req *http.Request) (interface{}, *HttpError) {
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
	err := metadataRepo.DeleteMapping(nodePath, subs...)
	if err != nil {
		return nil, NewServerError(err)
	} else {
		return nil, nil
	}
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

func rootHandler(req *http.Request) (interface{}, *HttpError) {
	clientIP := requestIP(req)
	vars := mux.Vars(req)
	nodePath := vars["nodePath"]
	if nodePath == "" {
		nodePath = "/"
	}
	wait := strings.ToLower(req.FormValue("wait")) == "true"
	var result interface{}
	if wait {
		change := strings.ToLower(req.FormValue("change")) != "false"
		result = metadataRepo.Watch(clientIP, nodePath)
		if !change {
			result = metadataRepo.Root(clientIP, nodePath)
		}
	} else {
		result = metadataRepo.Root(clientIP, nodePath)
	}
	if result == nil {
		return nil, NewHttpError(http.StatusNotFound, "Not found")
	} else {
		return result, nil
	}

}

func selfHandler(req *http.Request) (interface{}, *HttpError) {
	clientIP := requestIP(req)
	vars := mux.Vars(req)
	nodePath := vars["nodePath"]
	if nodePath == "" {
		nodePath = "/"
	}
	wait := strings.ToLower(req.FormValue("wait")) == "true"
	var result interface{}
	if wait {
		change := strings.ToLower(req.FormValue("change")) != "false"
		result = metadataRepo.WatchSelf(clientIP, nodePath)
		if !change {
			result = metadataRepo.Self(clientIP, nodePath)
		}
	} else {
		result = metadataRepo.Self(clientIP, nodePath)
	}
	if result == nil {
		return nil, NewHttpError(http.StatusNotFound, "Not found")
	} else {
		return result, nil
	}
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
	bytes, err := yaml.Marshal(val)
	if err == nil {
		w.Write(bytes)
	} else {
		respondError(w, req, "Error serializing to YAML: "+err.Error(), http.StatusInternalServerError)
	}
	return len(bytes)
}

func requestIP(req *http.Request) string {
	if config.EnableXff {
		clientIp := req.Header.Get("X-Forwarded-For")
		if len(clientIp) > 0 {
			return clientIp
		}
	}

	clientIp, _, _ := net.SplitHostPort(req.RemoteAddr)
	return clientIp
}

func handlerWrapper(handler handlerFunc) func(w http.ResponseWriter, req *http.Request) {

	return func(w http.ResponseWriter, req *http.Request) {
		start := time.Now().Nanosecond()
		result, err := handler(req)
		end := time.Now().Nanosecond()
		status := 200
		var len int
		if err != nil {
			status = err.Status
			respondError(w, req, err.Message, status)
			errorLog(req, status, err.Message)
		} else {
			if log.IsDebugEnable() {
				log.Debug("reponse success: %v", result)
			}
			if result == nil {
				respondSuccessDefault(w, req)
			} else {
				len = respondSuccess(w, req, result)
			}
		}
		requestLog(req, status, (end-start)/1000, len)
	}
}

func requestLog(req *http.Request, status int, time int, len int) {
	log.Info("REQ\t%s\t%s\t%s\t%v\t%v\t%v\t%v", req.Method, requestIP(req), req.RequestURI, req.ContentLength, status, time, len)
}

func errorLog(req *http.Request, status int, msg string) {
	if status == 500 {
		log.Error("ERR\t%s\t%s\t%s\t%v\t%v\t%s", req.Method, requestIP(req), req.RequestURI, req.ContentLength, status, msg)
	} else {
		log.Warning("ERR\t%s\t%s\t%s\t%v\t%v\t%s", req.Method, requestIP(req), req.RequestURI, req.ContentLength, status, msg)
	}
}
