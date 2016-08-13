package main

import (
	"flag"
	"fmt"
	"github.com/yunify/metadata-proxy/backends"
	"github.com/yunify/metadata-proxy/log"
	"github.com/yunify/metadata-proxy/metadata"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"strconv"
)

type Nodes []string

// String returns the string representation of a node var.
func (n *Nodes) String() string {
	return fmt.Sprintf("%s", *n)
}

// Set appends the node to the etcd node list.
func (n *Nodes) Set(node string) error {
	*n = append(*n, node)
	return nil
}

var (
	VERSION        string = "1.0"
	config         Config // holds the global config
	backendsConfig backends.Config
	storeClient    backends.StoreClient
	metadataRepo   *metadata.MetadataRepo

	printVersion bool
	logLevel     string
	enableXff    bool
	onlySelf     bool
	prefix       string
	listen       string
	listenManage string
	configFile   string
	pidFile      string

	backend      string
	basicAuth    bool
	clientCaKeys string
	clientCert   string
	clientKey    string
	nodes        Nodes
	username     string
	password     string
)

type Config struct {
	Backend      string                      `yaml:"backend"`
	LogLevel     string                      `yaml:"log_level"`
	PIDFile      string                      `yaml:"pid_file"`
	EnableXff    bool                        `yaml:"xff"`
	Prefix       string                      `yaml:"prefix"`
	OnlySelf     bool                        `yaml:"only_self"`
	Listen       string                      `yaml:"listen"`
	ListenManage string                      `yaml:"listen_manage"`
	BasicAuth    bool                        `yaml:"basic_auth"`
	ClientCaKeys string                      `yaml:"client_ca_keys"`
	ClientCert   string                      `yaml:"client_cert"`
	ClientKey    string                      `yaml:"client_key"`
	BackendNodes []string                    `yaml:"nodes"`
	Username     string                      `yaml:"username"`
	Password     string                      `yaml:"password"`
	SelfMapping  map[string]metadata.Mapping `yaml:"self_mapping"`
}

func init() {
	flag.BoolVar(&printVersion, "version", false, "Show version")
	flag.StringVar(&configFile, "config", "", "config file")
	flag.StringVar(&backend, "backend", "etcd", "backend to use")
	flag.StringVar(&logLevel, "log_level", "info", "set log level: debug|info|warning")
	flag.StringVar(&pidFile, "pid_file", "", "PID to write to")
	flag.BoolVar(&enableXff, "xff", false, "X-Forwarded-For header support")
	flag.StringVar(&prefix, "prefix", "", "default backend key prefix")
	flag.BoolVar(&onlySelf, "only_self", false, "only support self metadata query.")
	flag.StringVar(&listen, "listen", ":80", "Address to listen to (TCP)")
	flag.StringVar(&listenManage, "listen_manage", "127.0.0.1:8112", "Address to listen to for reload requests (TCP)")
	flag.BoolVar(&basicAuth, "basic_auth", false, "Use Basic Auth to authenticate (only used with -backend=etcd)")
	flag.StringVar(&clientCaKeys, "client_ca_keys", "", "client ca keys")
	flag.StringVar(&clientCert, "client_cert", "", "the client cert")
	flag.StringVar(&clientKey, "client_key", "", "the client key")
	flag.Var(&nodes, "node", "list of backend nodes")
	flag.StringVar(&username, "username", "", "the username to authenticate as (only used with etcd backends)")
	flag.StringVar(&password, "password", "", "the password to authenticate with (only used with etcd backends)")
}

func initConfig() error {

	// Set defaults.
	config = Config{
		Backend:      "etcd",
		Prefix:       "",
		LogLevel:     "info",
		Listen:       ":80",
		ListenManage: "127.0.0.1:8112",
		SelfMapping:  make(map[string]metadata.Mapping),
	}
	if configFile != "" {
		data, err := ioutil.ReadFile(configFile)
		if err != nil {
			log.Warning("Failed to read config file: %s, err: %s", configFile, err.Error())
			return err
		}
		err = yaml.Unmarshal(data, &config)
		if err != nil {
			log.Warning("Failed to parse config file: %s, err: %s", configFile, err.Error())
			return err
		}
	}

	// Update config from commandline flags.
	processFlags()

	if config.LogLevel != "" {
		println("set log level to:", config.LogLevel)
		log.SetLevel(config.LogLevel)
	}

	if config.PIDFile != "" {
		log.Info("Writing pid %d to %s", os.Getpid(), config.PIDFile)
		if err := ioutil.WriteFile(config.PIDFile, []byte(strconv.Itoa(os.Getpid())), 0644); err != nil {
			log.Fatal("Failed to write pid file %s: %v", config.PIDFile, err)
		}
	}

	if len(config.BackendNodes) == 0 {
		config.BackendNodes = backends.GetDefaultBackends(config.Backend)
	}

	backendsConfig = backends.Config{
		Backend:      config.Backend,
		BasicAuth:    config.BasicAuth,
		ClientCaKeys: config.ClientCaKeys,
		ClientCert:   config.ClientCert,
		ClientKey:    config.ClientKey,
		BackendNodes: config.BackendNodes,
		Password:     config.Password,
		Username:     config.Username,
	}
	return nil
}

// processFlags iterates through each flag set on the command line and
// overrides corresponding configuration settings.
func processFlags() {
	flag.Visit(setConfigFromFlag)
}

func setConfigFromFlag(f *flag.Flag) {
	println("process arg name:", f.Name, ", value:", f.Value.String(), "default:", f.DefValue)
	switch f.Name {
	case "backend":
		config.Backend = backend
	case "log_level":
		config.LogLevel = logLevel
	case "pid_file":
		config.PIDFile = pidFile
	case "xff":
		config.EnableXff = enableXff
	case "prefix":
		config.Prefix = prefix
	case "only_self":
		config.OnlySelf = onlySelf
	case "listen":
		config.Listen = listen
	case "listen_manage":
		config.ListenManage = listenManage
	case "basic_auth":
		config.BasicAuth = basicAuth
	case "client_cert":
		config.ClientCert = clientCert
	case "client_key":
		config.ClientKey = clientKey
	case "client_ca_keys":
		config.ClientCaKeys = clientCaKeys
	case "node":
		config.BackendNodes = nodes
	case "username":
		config.Username = username
	case "password":
		config.Password = password

	}
}
