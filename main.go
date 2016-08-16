package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"github.com/golang/gddo/httputil"
	"github.com/gorilla/mux"
	"github.com/yunify/metadata-proxy/backends"
	"github.com/yunify/metadata-proxy/log"
	"github.com/yunify/metadata-proxy/metadata"
	yaml "gopkg.in/yaml.v2"
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
}

func resync() error {
	metadataRepo.ReSync()
	return nil
}

func watchManage() {
	manageRouter := mux.NewRouter()
	manageRouter.HandleFunc("/favicon.ico", http.NotFound)
	manageRouter.HandleFunc("/v1/resync", httpResync).Methods("POST")
	manageRouter.HandleFunc("/v1/register", httpRegister).Methods("POST")
	manageRouter.HandleFunc("/v1/unregister", httpUnregister).Methods("POST")

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

func httpRegister(w http.ResponseWriter, req *http.Request) {
	log.Debug("Received HTTP register request")
	ip := req.FormValue("ip")
	mapping := make(metadata.Mapping)
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
	obj["type"] = "error"
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
