// Copyright 2018 Yunify Inc. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

package backends

import (
	"errors"
	"path"
	"strings"

	"github.com/yunify/metad/backends/etcdv3"
	"github.com/yunify/metad/backends/local"
	"github.com/yunify/metad/log"
	"github.com/yunify/metad/store"
)

// The StoreClient interface is implemented by objects that can retrieve
// key/value pairs from a backend store.
type StoreClient interface {
	Get(nodePath string, dir bool) (interface{}, error)
	Put(nodePath string, value interface{}, replace bool) error
	// Delete
	// if the 'key' represent a dir, 'dir' should be true.
	Delete(nodePath string, dir bool) error
	Sync(store store.Store, stopChan chan bool)

	GetMapping(nodePath string, dir bool) (interface{}, error)
	PutMapping(nodePath string, mapping interface{}, replace bool) error
	DeleteMapping(nodePath string, dir bool) error
	SyncMapping(mapping store.Store, stopChan chan bool)

	GetAccessRule() (map[string][]store.AccessRule, error)
	PutAccessRule(rules map[string][]store.AccessRule) error
	DeleteAccessRule(hosts []string) error
	SyncAccessRule(accessStore store.AccessStore, stopChan chan bool)
}

// New is used to create a storage client based on our configuration.
func New(config Config) (StoreClient, error) {
	if config.Backend == "" {
		config.Backend = "etcdv3"
	}
	if config.Group == "" {
		config.Group = "default"
	}
	config.Prefix = path.Join("/", config.Prefix)
	backendNodes := config.BackendNodes
	log.Info("Backend nodes set to " + strings.Join(backendNodes, ", "))
	if len(backendNodes) == 0 {
		backendNodes = GetDefaultBackends(config.Backend)
	}
	switch config.Backend {
	case "etcd", "etcdv3":
		// Create the etcdv3 client upfront and use it for the life of the process.
		return etcdv3.NewEtcdClient(config.Group, config.Prefix, backendNodes, config.ClientCert, config.ClientKey, config.ClientCaKeys, config.BasicAuth, config.Username, config.Password)
	case "local":
		return local.NewLocalClient()
	}

	return nil, errors.New("Invalid backend")
}

func GetDefaultBackends(backend string) []string {
	switch backend {
	case "etcd", "etcdv3":
		return []string{"http://127.0.0.1:2379"}
	default:
		return nil
	}
}
