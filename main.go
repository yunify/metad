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

	log "github.com/Sirupsen/logrus"
	"github.com/golang/gddo/httputil"
	"github.com/gorilla/mux"
	"github.com/yunify/metadata-proxy/backends"
	"github.com/yunify/metadata-proxy/metadata"
	"github.com/yunify/metadata-proxy/store"
	yaml "gopkg.in/yaml.v2"
)

const (
	ContentText = 1
	ContentJSON = 2
	ContentYAML = 3
)

var (
	router = mux.NewRouter()

	reloadChan = make(chan chan error)
)

func main() {
	parseFlags()

	log.Infof("Starting metadata-proxy %s", VERSION)
	var err error
	storeClient, err = backends.New(backendsConfig)
	if err != nil {
		log.Fatal(err.Error())
	}
	metastore := store.New()
	metadataRepo = metadata.New(config.Prefix, config.SelfMapping, storeClient, metastore)

	metadataRepo.StartSync()

	watchSignals()
	watchHttp()

	router.HandleFunc("/favicon.ico", http.NotFound)

	router.HandleFunc("/self/{key:.*}", selfHandler).
		Methods("GET", "HEAD").
		Name("Self")

	router.HandleFunc("/{key:.*}", metadataHandler).
		Methods("GET", "HEAD").
		Name("Metadata")

	log.Info("Listening on ", config.Listen)
	log.Fatal(http.ListenAndServe(config.Listen, router))
}

func parseFlags() {
	flag.Parse()

	if printVersion {
		fmt.Printf("%s\n", VERSION)
		os.Exit(0)
	}

	if err := initConfig(); err != nil {
		log.Fatal(err.Error())
		os.Exit(-1)
	}
}

func watchSignals() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP)

	go func() {
		for _ = range c {
			log.Info("Received HUP signal")
			reloadChan <- nil
		}
	}()

	go func() {
		for resp := range reloadChan {
			err := reload()
			if resp != nil {
				resp <- err
			}
		}
	}()
}

func reload() error {
	//TODO
	return nil
}

func watchHttp() {
	reloadRouter := mux.NewRouter()
	reloadRouter.HandleFunc("/favicon.ico", http.NotFound)
	reloadRouter.HandleFunc("/v1/reload", httpReload).Methods("POST")

	log.Info("Listening for Reload on ", config.ListenManage)
	go http.ListenAndServe(config.ListenManage, reloadRouter)
}

func httpReload(w http.ResponseWriter, req *http.Request) {
	log.Debugf("Received HTTP reload request")
	respChan := make(chan error)
	reloadChan <- respChan
	err := <-respChan

	if err == nil {
		io.WriteString(w, "OK")
	} else {
		w.WriteHeader(500)
		io.WriteString(w, err.Error())
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

func metadataHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	clientIp := requestIp(req)
	requestPath := strings.TrimRight(req.URL.EscapedPath()[1:], "/")

	if config.OnlySelf {
		if requestPath == "/" {
			selfVal, ok := metadataRepo.GetSelf(clientIp, "/")
			if ok {
				val := make(map[string]interface{})
				val["self"] = selfVal
				respondSuccess(w, req, val)
			} else {
				respondSuccess(w, req, make(map[string]interface{}))
			}
		} else {
			respondError(w, req, "Not found", http.StatusNotFound)
		}
	} else {
		val, ok := metadataRepo.Get(requestPath)
		if !ok {
			log.WithFields(log.Fields{"client": clientIp}).Warningf("self not found %s", clientIp)
			respondError(w, req, "Not found", http.StatusNotFound)
		} else {
			if requestPath == "/" {
				selfVal, ok := metadataRepo.GetSelf(clientIp, "/")
				if ok {
					mapVal, ok := val.(map[string]interface{})
					if ok {
						mapVal["self"] = selfVal
					}
				}
			}
			respondSuccess(w, req, val)
		}

	}

}

func selfHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	clientIp := requestIp(req)

	requestPath := strings.TrimRight(req.URL.EscapedPath()[1:], "/")

	val, ok := metadataRepo.GetSelf(clientIp, requestPath)
	if !ok {
		log.WithFields(log.Fields{"client": clientIp}).Warningf("self not found %s", clientIp)
		respondError(w, req, "Not found", http.StatusNotFound)
	} else {
		log.WithFields(log.Fields{"client": clientIp}).Infof("/self/%s OK", requestPath)
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
	log.Infof("reponse success %v", val)
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

func requestIp(req *http.Request) string {
	if config.EnableXff {
		clientIp := req.Header.Get("X-Forwarded-For")
		if len(clientIp) > 0 {
			return clientIp
		}
	}

	clientIp, _, _ := net.SplitHostPort(req.RemoteAddr)
	return clientIp
}
