// Copyright 2018 Yunify Inc. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	"gopkg.in/yaml.v2"

	"github.com/yunify/metad/backends"
	"github.com/yunify/metad/log"
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
	metad *Metad

	printVersion bool
	pprof        bool
	logLevel     string
	enableXff    bool
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
	group        string
)

type Config struct {
	Backend      string   `yaml:"backend"`
	LogLevel     string   `yaml:"log_level"`
	PIDFile      string   `yaml:"pid_file"`
	EnableXff    bool     `yaml:"xff"`
	Prefix       string   `yaml:"prefix"`
	Listen       string   `yaml:"listen"`
	ListenManage string   `yaml:"listen_manage"`
	BasicAuth    bool     `yaml:"basic_auth"`
	ClientCaKeys string   `yaml:"client_ca_keys"`
	ClientCert   string   `yaml:"client_cert"`
	ClientKey    string   `yaml:"client_key"`
	BackendNodes []string `yaml:"nodes"`
	Username     string   `yaml:"username"`
	Password     string   `yaml:"password"`
	Group        string   `yaml:"Group"`
}

func init() {
	flag.BoolVar(&printVersion, "version", false, "Show metad version")
	flag.BoolVar(&pprof, "pprof", false, "Enable http pprof, port is 6060")
	flag.StringVar(&configFile, "config", "", "The configuration file path")
	flag.StringVar(&backend, "backend", "local", "The metad backend type")
	flag.StringVar(&logLevel, "log_level", "info", "Log level for metad print out: debug|info|warning")
	flag.StringVar(&pidFile, "pid_file", "", "PID to write to")
	flag.BoolVar(&enableXff, "xff", false, "X-Forwarded-For header support")
	flag.StringVar(&prefix, "prefix", "", "Backend key path prefix")
	flag.StringVar(&group, "group", "default", "The metad's group name, same group share same mapping config from backend")
	flag.StringVar(&listen, "listen", ":80", "Address to listen to (TCP)")
	flag.StringVar(&listenManage, "listen_manage", "127.0.0.1:9611", "Address to listen to for manage requests (TCP)")
	flag.BoolVar(&basicAuth, "basic_auth", false, "Use Basic Auth to authenticate (only used with -backend=etcd)")
	flag.StringVar(&clientCaKeys, "client_ca_keys", "", "The client ca keys")
	flag.StringVar(&clientCert, "client_cert", "", "The client cert")
	flag.StringVar(&clientKey, "client_key", "", "The client key")
	flag.Var(&nodes, "nodes", "List of backend nodes")
	flag.StringVar(&username, "username", "", "The username to authenticate as (only used with etcd backends)")
	flag.StringVar(&password, "password", "", "The password to authenticate with (only used with etcd backends)")
}

func initConfig() (*Config, error) {

	// Set defaults.
	config := &Config{
		Backend:      "local",
		Prefix:       "",
		Group:        "default",
		LogLevel:     "info",
		Listen:       ":80",
		ListenManage: "127.0.0.1:9611",
	}
	if configFile != "" {
		err := loadConfigFile(configFile, config)
		if err != nil {
			return nil, err
		}
	}

	// Update config from commandline flags.
	processFlags(config)

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

	return config, nil
}

func loadConfigFile(configFile string, config *Config) error {
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Warning("Failed to read config file: %s, err: %s", configFile, err.Error())
		return err
	}
	err = yaml.Unmarshal(data, config)
	if err != nil {
		log.Warning("Failed to parse config file: %s, err: %s", configFile, err.Error())
		return err
	}
	return nil
}

// processFlags iterates through each flag set on the command line and
// overrides corresponding configuration settings.
func processFlags(config *Config) {
	flag.Visit(func(f *flag.Flag) {
		setConfigFromFlag(config, f)
	})
}

func setConfigFromFlag(config *Config, f *flag.Flag) {
	fmt.Printf("process arg name: %s, value: %s, default: %s\n", f.Name, f.Value.String(), f.DefValue)
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
	case "group":
		config.Group = group
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
	case "nodes":
		config.BackendNodes = nodes
	case "username":
		config.Username = username
	case "password":
		config.Password = password
	}
}
