package etcd

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/coreos/etcd/client"
	"github.com/yunify/metad/log"
	"github.com/yunify/metad/store"
	"github.com/yunify/metad/util"
	"github.com/yunify/metad/util/flatmap"
	"golang.org/x/net/context"
	"io/ioutil"
	"net"
	"net/http"
	"path"
	"reflect"
	"time"
)

const SELF_MAPPING_PATH = "/_metad/mapping"

// Client is a wrapper around the etcd client
type Client struct {
	client        client.KeysAPI
	prefix        string
	mappingPrefix string
}

// NewEtcdClient returns an *etcd.Client with a connection to named machines.
func NewEtcdClient(group string, prefix string, machines []string, cert, key, caCert string, basicAuth bool, username string, password string) (*Client, error) {
	var c client.Client
	var kapi client.KeysAPI
	var err error
	var transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
	}

	cfg := client.Config{
		Endpoints:               machines,
		HeaderTimeoutPerRequest: time.Duration(3) * time.Second,
	}

	if basicAuth {
		cfg.Username = username
		cfg.Password = password
	}

	if caCert != "" {
		certBytes, err := ioutil.ReadFile(caCert)
		if err != nil {
			return nil, err
		}

		caCertPool := x509.NewCertPool()
		ok := caCertPool.AppendCertsFromPEM(certBytes)

		if ok {
			tlsConfig.RootCAs = caCertPool
		}
	}

	if cert != "" && key != "" {
		tlsCert, err := tls.LoadX509KeyPair(cert, key)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{tlsCert}
	}

	transport.TLSClientConfig = tlsConfig
	cfg.Transport = transport

	c, err = client.New(cfg)
	if err != nil {
		return nil, err
	}

	kapi = client.NewKeysAPI(c)
	return &Client{kapi, prefix, path.Join(SELF_MAPPING_PATH, group)}, nil
}

// Get queries etcd for nodePath. Dir for query is recursive.
func (c *Client) Get(nodePath string, dir bool) (interface{}, error) {
	if dir {
		m, err := c.internalGets(c.prefix, nodePath)
		if err != nil {
			return nil, err
		}
		return flatmap.Expand(m, nodePath), nil
	} else {
		return c.internalGet(c.prefix, nodePath)
	}
}

func (c *Client) Put(nodePath string, value interface{}, replace bool) error {
	return c.internalPut(c.prefix, nodePath, value, replace)
}

func (c *Client) Delete(nodePath string, dir bool) error {
	return c.internalDelete(c.prefix, nodePath, dir)
}

func (c *Client) Sync(store store.Store, stopChan chan bool) {
	go c.internalSync(c.prefix, store, stopChan)
}

func (c *Client) GetMapping(nodePath string, dir bool) (interface{}, error) {
	if dir {
		m, err := c.internalGets(c.mappingPrefix, nodePath)
		if err != nil {
			return nil, err
		}
		return flatmap.Expand(m, nodePath), nil
	} else {
		return c.internalGet(c.mappingPrefix, nodePath)
	}
}

func (c *Client) PutMapping(nodePath string, mapping interface{}, replace bool) error {
	return c.internalPut(c.mappingPrefix, nodePath, mapping, replace)
}

func (c *Client) SyncMapping(mapping store.Store, stopChan chan bool) {
	go c.internalSync(c.mappingPrefix, mapping, stopChan)
}

func (c *Client) DeleteMapping(nodePath string, dir bool) error {
	nodePath = path.Join("/", nodePath)
	return c.internalDelete(c.mappingPrefix, nodePath, dir)
}

func (c *Client) internalGets(prefix, nodePath string) (map[string]string, error) {
	vars := make(map[string]string)
	nodePath = util.AppendPathPrefix(nodePath, prefix)
	resp, err := c.client.Get(context.Background(), nodePath, &client.GetOptions{
		Recursive: true,
		Sort:      true,
		Quorum:    true,
	})
	if err != nil {
		switch e := err.(type) {
		case client.Error:
			//if nodePath is not exist, just return empty map.
			if e.Code == client.ErrorCodeKeyNotFound {
				log.Warning("GetValues nodePath:%s ErrorCodeKeyNotFound", nodePath)
				return make(map[string]string), nil
			}
		}
		return nil, err
	}
	err = nodeWalk(prefix, resp.Node, vars)
	if err != nil {
		return nil, err
	}
	log.Debug("GetValues nodePath:%s, values:%v", nodePath, vars)
	return vars, nil
}

// nodeWalk recursively descends nodes, updating vars.
func nodeWalk(prefix string, node *client.Node, vars map[string]string) error {
	if node != nil {
		nodePath := node.Key
		if !node.Dir {
			nodePath = util.TrimPathPrefix(nodePath, prefix)
			vars[nodePath] = node.Value
		} else {
			for _, node := range node.Nodes {
				nodeWalk(prefix, node, vars)
			}
		}
	}
	return nil
}

func (c *Client) internalGet(prefix, nodePath string) (string, error) {
	resp, err := c.client.Get(context.Background(), util.AppendPathPrefix(nodePath, prefix), nil)
	if err != nil {
		switch e := err.(type) {
		case client.Error:
			//if nodePath is not exist, just return empty str.
			if e.Code == client.ErrorCodeKeyNotFound {
				return "", nil
			}
		}
		return "", err
	}
	return resp.Node.Value, nil
}

func (c *Client) internalSync(prefix string, store store.Store, stopChan chan bool) {

	var waitIndex uint64 = 0
	init := false
	cancelRoutine := make(chan bool)
	defer close(cancelRoutine)

	for {
		select {
		case <-cancelRoutine:
			return
		default:
		}

		watcher := c.client.Watcher(prefix, &client.WatcherOptions{AfterIndex: waitIndex, Recursive: true})
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			select {
			case <-stopChan:
				log.Info("Sync %s stop.", prefix)
				cancel()
				cancelRoutine <- true
				return
			}
		}()

		for !init {
			val, err := c.internalGets(prefix, "/")
			if err != nil {
				log.Error("GetValue from etcd nodePath:%s, error-type: %s, error: %s", prefix, reflect.TypeOf(err), err.Error())
				switch e := err.(type) {
				case client.Error:
					//if nodePath of prefix is not exist, just create a empty dir.
					if e.Code == client.ErrorCodeKeyNotFound {
						resp, createErr := c.client.Set(context.Background(), prefix, "", &client.SetOptions{
							Dir: true,
						})
						if createErr != nil {
							log.Error("Create dir %s error: %s", prefix, createErr.Error())
						} else {
							log.Info("Create dir %s resp: %v", prefix, resp)
						}
					}
				}
				log.Info("Init store for prefix %s fail, retry.", prefix)
				time.Sleep(time.Duration(1000) * time.Millisecond)
				continue
			}
			store.PutBulk("/", val)
			log.Info("Init store for prefix %s success.", prefix)
			init = true
		}
		resp, err := watcher.Next(ctx)
		if err != nil {
			log.Error("Watch etcd error: %s", err.Error())
			time.Sleep(time.Duration(1000) * time.Millisecond)
			continue
		}
		processSyncChange(prefix, store, resp)
		waitIndex = resp.Node.ModifiedIndex
	}
}

func processSyncChange(prefix string, store store.Store, resp *client.Response) {
	nodePath := util.TrimPathPrefix(resp.Node.Key, prefix)
	log.Debug("process sync change, prefix: %v, nodePath:%v, resp: %v ", prefix, nodePath, resp)
	//TODO wait etcd 3.1.0 support watch children dir action. https://github.com/coreos/etcd/issues/1229
	switch resp.Action {
	case "delete":
		store.Delete(nodePath)
	default:
		store.Put(nodePath, resp.Node.Value)
	}
}

func (c *Client) internalPut(prefix, nodePath string, value interface{}, replace bool) error {
	switch t := value.(type) {
	case map[string]interface{}, map[string]string, []interface{}:
		flatValues := flatmap.Flatten(t)
		return c.internalPutValues(prefix, nodePath, flatValues, replace)
	default:
		val := fmt.Sprintf("%v", t)
		return c.internalPutValue(prefix, nodePath, val)
	}
}

func (c *Client) internalPutValue(prefix string, nodePath string, value string) error {
	nodePath = util.AppendPathPrefix(nodePath, prefix)
	resp, err := c.client.Set(context.TODO(), nodePath, value, nil)
	log.Debug("SetValue nodePath: %s, value:%s, resp:%v", nodePath, value, resp)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) internalPutValues(prefix string, nodePath string, values map[string]string, replace bool) error {
	if replace {
		c.internalDelete(prefix, nodePath, true)
	}
	new_prefix := util.AppendPathPrefix(nodePath, prefix)
	for k, v := range values {
		k = util.AppendPathPrefix(k, new_prefix)
		resp, err := c.client.Set(context.TODO(), k, v, nil)
		log.Debug("SetValue nodePath:%s, value:%s, resp:%v", k, v, resp)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) internalDelete(prefix, nodePath string, dir bool) error {
	nodePath = util.AppendPathPrefix(nodePath, prefix)
	log.Debug("Delete from backend, nodePath:%s, dir:%v", nodePath, dir)
	_, err := c.client.Delete(context.Background(), nodePath, &client.DeleteOptions{Recursive: dir})
	return err
}
