package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/golang/gddo/httputil"
	"github.com/gorilla/mux"
	"github.com/yunify/metadata-proxy/backends"
	"github.com/yunify/metadata-proxy/log"
	"github.com/yunify/metadata-proxy/metadata"
	yaml "gopkg.in/yaml.v2"
	"io"
	"net"
	"net/http"
	"net/url"
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

	log.Info("Starting metadata-proxy %s", VERSION)
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

	router.HandleFunc("/self", selfHandler).
		Methods("GET", "HEAD").
		Name("SelfRoot")

	router.HandleFunc("/self/{key:.*}", selfHandler).
		Methods("GET", "HEAD").
		Name("Self")

	router.HandleFunc("/{key:.*}", metadataHandler).
		Methods("GET", "HEAD").
		Name("Metadata")

	log.Info("Listening on %s", config.Listen)
	log.Fatal("%v", http.ListenAndServe(config.Listen, router))
}

func watchSignals() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP)

	go func() {
		for _ = range c {
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
	//manageRouter.HandleFunc("/v1/resync", httpResync).Methods("POST")
	manageRouter.HandleFunc("/v1/register", httpRegister).Methods("POST")
	manageRouter.HandleFunc("/v1/unregister", httpUnregister).Methods("POST")

	v1 := manageRouter.PathPrefix("/v1").Subrouter()
	v1.HandleFunc("/resync", httpResync).Methods("POST")

	v1.HandleFunc("/mapping", mappingGet).Methods("GET")
	v1.HandleFunc("/mapping", mappingUpdate).Methods("POST", "PUT")
	v1.HandleFunc("/mapping", mappingDelete).Methods("DELETE")

	mapping := v1.PathPrefix("/mapping").Subrouter()
	//mapping.HandleFunc("", mappingGET).Methods("GET")
	mapping.HandleFunc("/{nodePath:.*}", mappingGet).Methods("GET")
	mapping.HandleFunc("/{nodePath:.*}", mappingUpdate).Methods("POST", "PUT")
	mapping.HandleFunc("/{nodePath:.*}", mappingDelete).Methods("DELETE")

	v1.HandleFunc("/data", dataGet).Methods("GET")
	v1.HandleFunc("/data", dataUpdate).Methods("POST", "PUT")
	v1.HandleFunc("/data", dataDelete).Methods("DELETE")

	data := v1.PathPrefix("/data").Subrouter()
	//mapping.HandleFunc("", mappingGET).Methods("GET")
	data.HandleFunc("/{nodePath:.*}", dataGet).Methods("GET")
	data.HandleFunc("/{nodePath:.*}", dataUpdate).Methods("POST", "PUT")
	data.HandleFunc("/{nodePath:.*}", dataDelete).Methods("DELETE")

	log.Info("Listening for Manage on %s", config.ListenManage)
	go http.ListenAndServe(config.ListenManage, manageRouter)
}

func httpResync(w http.ResponseWriter, req *http.Request) {
	log.Debug("Received HTTP resync request")
	respChan := make(chan error)
	resyncChan <- respChan
	err := <-respChan

	if err == nil {
		io.WriteString(w, "OK")
	} else {
		w.WriteHeader(500)
		io.WriteString(w, err.Error())
	}
}

func getNodePath(requestURI string) string {
	//trim the v1 and router path
	parts := strings.Split(requestURI, "/")
	nodePath := "/" + strings.Join(parts[3:], "/")
	return nodePath
}

func dataGet(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	nodePath := vars["nodePath"]
	if nodePath == "" {
		nodePath = "/"
	}
	val, ok := metadataRepo.GetData(nodePath)
	if !ok {
		log.Warning("dataGet %s not found", nodePath)
		respondError(w, req, "Not found", http.StatusNotFound)
	} else {
		log.Info("dataGet %s OK", nodePath)
		respondSuccess(w, req, val)
	}
}

func dataUpdate(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	nodePath := vars["nodePath"]
	if nodePath == "" {
		nodePath = "/"
	}
	decoder := json.NewDecoder(req.Body)
	var data interface{}
	err := decoder.Decode(&data)
	if err != nil {
		respondError(w, req, fmt.Sprintf("invalid json format, error:%s", err.Error()), 400)
	} else {
		// POST means replace old value
		// PUT means merge to old value
		replace := "POST" == strings.ToUpper(req.Method)
		err = metadataRepo.UpdateData(nodePath, data, replace)
		if err != nil {
			msg := fmt.Sprintf("Update data error:%s", err.Error())
			log.Error("dataUpdate  nodePath:%s, data:%v, error:%s", nodePath, data, err.Error())
			respondError(w, req, msg, http.StatusInternalServerError)
		} else {
			log.Info("dataUpdate %s OK", nodePath)
			respondSuccessDefault(w, req)
		}
	}
}

func dataDelete(w http.ResponseWriter, req *http.Request) {
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
		msg := fmt.Sprintf("Delete data error:%s", err.Error())
		log.Error("dataDelete  nodePath:%s, error:%s", nodePath, err.Error())
		respondError(w, req, msg, http.StatusInternalServerError)
	} else {
		log.Info("dataDelete %s OK", nodePath)
		respondSuccessDefault(w, req)
	}
}

func mappingGet(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	nodePath := vars["nodePath"]
	if nodePath == "" {
		nodePath = "/"
	}
	val, ok := metadataRepo.GetMapping(nodePath)
	if !ok {
		log.Warning("mappingGet %s not found", nodePath)
		respondError(w, req, "Not found", http.StatusNotFound)
	} else {
		log.Info("mappingGet %s OK", nodePath)
		respondSuccess(w, req, val)
	}
}

func mappingUpdate(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	nodePath := vars["nodePath"]
	if nodePath == "" {
		nodePath = "/"
	}
	decoder := json.NewDecoder(req.Body)
	var data interface{}
	err := decoder.Decode(&data)
	if err != nil {
		respondError(w, req, fmt.Sprintf("invalid json format, error:%s", err.Error()), 400)
	} else {
		// POST means replace old value
		// PUT means merge to old value
		replace := "POST" == strings.ToUpper(req.Method)
		err = metadataRepo.UpdateMapping(nodePath, data, replace)
		if err != nil {
			msg := fmt.Sprintf("Update mapping error:%s", err.Error())
			log.Error("mappingUpdate  nodePath:%s, data:%v, error:%s", nodePath, data, err.Error())
			respondError(w, req, msg, http.StatusInternalServerError)
		} else {
			log.Info("mappingUpdate %s OK", nodePath)
			respondSuccessDefault(w, req)
		}
	}
}

func mappingDelete(w http.ResponseWriter, req *http.Request) {
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
		msg := fmt.Sprintf("Delete mapping error:%s", err.Error())
		log.Error("mappingDelete  nodePath:%s, error:%s", nodePath, err.Error())
		respondError(w, req, msg, http.StatusInternalServerError)
	} else {
		log.Info("mappingDelete %s OK", nodePath)
		respondSuccessDefault(w, req)
	}
}

func httpMapping(w http.ResponseWriter, req *http.Request) {
	log.Debug("Received HTTP mapping request")
	ip := req.FormValue("ip")
	mapping := make(map[string]string)
	mapppingstr := req.FormValue("mapping")
	err := json.Unmarshal([]byte(mapppingstr), &mapping)
	metadataRepo.Register(ip, mapping)
	if err == nil {
		io.WriteString(w, "OK")
	} else {
		w.WriteHeader(500)
		io.WriteString(w, err.Error())
	}
}

func httpRegister(w http.ResponseWriter, req *http.Request) {
	log.Debug("Received HTTP register request")
	ip := req.FormValue("ip")
	mapping := make(map[string]string)
	mapppingstr := req.FormValue("mapping")
	err := json.Unmarshal([]byte(mapppingstr), &mapping)
	metadataRepo.Register(ip, mapping)
	if err == nil {
		io.WriteString(w, "OK")
	} else {
		w.WriteHeader(500)
		io.WriteString(w, err.Error())
	}
}

func httpUnregister(w http.ResponseWriter, req *http.Request) {
	log.Debug("Received HTTP register request")
	ip := req.FormValue("ip")
	metadataRepo.Unregister(ip)
	io.WriteString(w, "OK")
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

func metadataHandler(w http.ResponseWriter, req *http.Request) {
	clientIP := requestIP(req)
	requestPath := req.URL.EscapedPath() //strings.TrimRight(req.URL.EscapedPath()[1:], "/")
	log.Debug("clientIP: %s, requestPath: %s", clientIP, requestPath)

	val, ok := metadataRepo.Get(clientIP, requestPath)
	if !ok {
		log.Warning("%s not found %s", requestPath, clientIP)
		respondError(w, req, "Not found", http.StatusNotFound)
	} else {
		log.Info("%s %s OK", requestPath, clientIP)
		respondSuccess(w, req, val)
	}

}

func selfHandler(w http.ResponseWriter, req *http.Request) {
	clientIP := requestIP(req)
	requestPath := strings.TrimLeft(req.URL.EscapedPath(), "/self")

	val, ok := metadataRepo.GetSelf(clientIP, requestPath)
	if !ok {
		log.Warning("self not found %s", clientIP)
		respondError(w, req, "Not found", http.StatusNotFound)
	} else {
		log.Info("/self/%s %s OK", requestPath, clientIP)
		respondSuccess(w, req, val)
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

func respondSuccess(w http.ResponseWriter, req *http.Request, val interface{}) {
	log.Info("reponse success %v", val)
	switch contentType(req) {
	case ContentText:
		respondText(w, req, val)
	case ContentJSON:
		respondJSON(w, req, val)
	case ContentYAML:
		respondYAML(w, req, val)
	}
}

func respondText(w http.ResponseWriter, req *http.Request, val interface{}) {
	if val == nil {
		fmt.Fprint(w, "")
		return
	}

	switch v := val.(type) {
	case string:
		fmt.Fprint(w, v)
	case uint, uint8, uint16, uint32, uint64, int, int8, int16, int32, int64:
		fmt.Fprintf(w, "%d", v)
	case float64:
		// The default format has extra trailing zeros
		str := strings.TrimRight(fmt.Sprintf("%f", v), "0")
		str = strings.TrimRight(str, ".")
		fmt.Fprint(w, str)
	case bool:
		if v {
			fmt.Fprint(w, "true")
		} else {
			fmt.Fprint(w, "false")
		}
	case map[string]interface{}:
		out := make([]string, len(v))
		i := 0
		for k, vv := range v {
			_, isMap := vv.(map[string]interface{})
			_, isArray := vv.([]interface{})
			if isMap || isArray {
				out[i] = fmt.Sprintf("%s/\n", url.QueryEscape(k))
			} else {
				out[i] = fmt.Sprintf("%s\n", url.QueryEscape(k))
			}
			i++
		}

		sort.Strings(out)
		for _, vv := range out {
			fmt.Fprint(w, vv)
		}
	default:
		http.Error(w, "Value is of a type I don't know how to handle", http.StatusInternalServerError)
	}
}

func respondJSON(w http.ResponseWriter, req *http.Request, val interface{}) {
	bytes, err := json.Marshal(val)
	if err == nil {
		w.Write(bytes)
	} else {
		respondError(w, req, "Error serializing to JSON: "+err.Error(), http.StatusInternalServerError)
	}
}

func respondYAML(w http.ResponseWriter, req *http.Request, val interface{}) {
	bytes, err := yaml.Marshal(val)
	if err == nil {
		w.Write(bytes)
	} else {
		respondError(w, req, "Error serializing to YAML: "+err.Error(), http.StatusInternalServerError)
	}
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
