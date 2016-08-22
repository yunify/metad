package etcd

import (
	"crypto/tls"
	"crypto/x509"
	"github.com/coreos/etcd/client"
	"github.com/yunify/metadata-proxy/log"
	"github.com/yunify/metadata-proxy/store"
	"github.com/yunify/metadata-proxy/util"
	"github.com/yunify/metadata-proxy/util/flatmap"
	"golang.org/x/net/context"
	"io/ioutil"
	"net"
	"net/http"
	"reflect"
	"time"
)

const SELF_MAPPING_PATH = "/_metadata-proxy/mapping"

// Client is a wrapper around the etcd client
type Client struct {
	client client.KeysAPI
	prefix string
}

// NewEtcdClient returns an *etcd.Client with a connection to named machines.
func NewEtcdClient(prefix string, machines []string, cert, key, caCert string, basicAuth bool, username string, password string) (*Client, error) {
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
			return &Client{kapi, prefix}, err
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
			return &Client{kapi, prefix}, err
		}
		tlsConfig.Certificates = []tls.Certificate{tlsCert}
	}

	transport.TLSClientConfig = tlsConfig
	cfg.Transport = transport

	c, err = client.New(cfg)
	if err != nil {
		return &Client{kapi, prefix}, err
	}

	kapi = client.NewKeysAPI(c)
	return &Client{kapi, prefix}, nil
}

// GetValues queries etcd for key Recursive:true.
func (c *Client) GetValues(key string) (map[string]interface{}, error) {
	m, err := c.internalGetValues(c.prefix, key)
	if err != nil {
		return nil, err
	}
	return flatmap.Expand(m, util.AppendPathPrefix(key, c.prefix)), nil
}

func (c *Client) internalGetValues(prefix, key string) (map[string]string, error) {
	vars := make(map[string]string)
	key = util.AppendPathPrefix(key, prefix)
	resp, err := c.client.Get(context.Background(), key, &client.GetOptions{
		Recursive: true,
		Sort:      true,
		Quorum:    true,
	})
	if err != nil {
		switch e := err.(type) {
		case client.Error:
			//if key is not exist, just return empty map.
			if e.Code == client.ErrorCodeKeyNotFound {
				log.Warning("GetValues key:%s ErrorCodeKeyNotFound", key)
				return make(map[string]string), nil
			}
		}
		return nil, err
	}
	err = nodeWalk(resp.Node, vars)
	if err != nil {
		return nil, err
	}
	log.Debug("GetValues key:%s, values:%v", key, vars)
	return vars, nil
}

// nodeWalk recursively descends nodes, updating vars.
func nodeWalk(node *client.Node, vars map[string]string) error {
	if node != nil {
		key := node.Key
		if !node.Dir {
			//key = util.TrimPathPrefix(key, prefix)
			vars[key] = node.Value
		} else {
			for _, node := range node.Nodes {
				nodeWalk(node, vars)
			}
		}
	}
	return nil
}

// GetValue queries etcd for key
func (c *Client) GetValue(key string) (string, error) {
	return c.internalGetValue(c.prefix, key)
}

func (c *Client) internalGetValue(prefix, key string) (string, error) {
	resp, err := c.client.Get(context.Background(), util.AppendPathPrefix(key, prefix), nil)
	if err != nil {
		switch e := err.(type) {
		case client.Error:
			//if key is not exist, just return empty str.
			if e.Code == client.ErrorCodeKeyNotFound {
				return "", nil
			}
		}
		return "", err
	}
	return resp.Node.Value, nil
}

func (c *Client) internalSync(prefix string, store store.Store, stopChan chan bool) {

	defer func() {
		if r := recover(); r != nil {
			log.Error("Sync Recover: %v, try restart.", r)
			time.Sleep(time.Duration(1000) * time.Millisecond)
			c.internalSync(prefix, store, stopChan)
		}
	}()

	var waitIndex uint64 = 0
	inited := false
	for {
		watcher := c.client.Watcher(prefix, &client.WatcherOptions{AfterIndex: waitIndex, Recursive: true})
		ctx, cancel := context.WithCancel(context.Background())
		cancelRoutine := make(chan bool)
		defer close(cancelRoutine)

		go func() {
			select {
			case <-stopChan:
				cancel()
			case <-cancelRoutine:
				return
			}
		}()

		for !inited {
			val, err := c.GetValues("/")
			if err != nil {
				log.Error("GetValue from etcd key:%s, error-type: %s, error: %s", prefix, reflect.TypeOf(err), err.Error())
				switch e := err.(type) {
				case client.Error:
					//if key of prefix is not exist, just create a empty dir.
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
				time.Sleep(time.Duration(1000) * time.Millisecond)
				continue
			}
			store.Sets("/", val)
			inited = true
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
	key := util.TrimPathPrefix(resp.Node.Key, prefix)
	log.Debug("process sync change, prefix: %v, key:%v, resp: %v ", prefix, key, resp)
	//TODO wait etcd 3.1.0 support watch children dir action. https://github.com/coreos/etcd/issues/1229
	switch resp.Action {
	case "delete":
		store.Delete(key)
	default:
		store.Set(key, false, resp.Node.Value)
	}
}

func (c *Client) Sync(store store.Store, stopChan chan bool) {
	go c.internalSync(c.prefix, store, stopChan)
}

func (c *Client) SetValues(key string, values map[string]interface{}, replace bool) error {
	flatValue := flatmap.Flatten(values)
	return c.internalSetValues(c.prefix, key, flatValue, replace)
}

func (c *Client) internalSetValues(prefix string, key string, values map[string]string, replace bool) error {
	if replace {
		c.internalDelete(prefix, key, true)
	}
	new_prefix := util.AppendPathPrefix(key, prefix)
	for k, v := range values {
		k = util.AppendPathPrefix(k, new_prefix)
		resp, err := c.client.Set(context.TODO(), k, v, nil)
		log.Debug("SetValue key:%s, value:%s, resp:%v", k, v, resp)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) SetValue(key string, value string) error {
	return c.internalSetValue(c.prefix, key, value)
}

func (c *Client) internalSetValue(prefix string, key string, value string) error {
	key = util.AppendPathPrefix(key, prefix)
	resp, err := c.client.Set(context.TODO(), key, value, nil)
	log.Debug("SetValue key: %s, value:%s, resp:%v", key, value, resp)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) Delete(key string, dir bool) error {
	return c.internalDelete(c.prefix, key, dir)
}

func (c *Client) internalDelete(prefix, key string, dir bool) error {
	key = util.AppendPathPrefix(key, prefix)
	log.Debug("Delete from backend, key:%s, dir:%v", key, dir)
	_, err := c.client.Delete(context.Background(), key, &client.DeleteOptions{Recursive: dir})
	return err
}

func (c *Client) SyncSelfMapping(mapping store.Store, stopChan chan bool) {
	go c.internalSync(SELF_MAPPING_PATH, mapping, stopChan)
}

func (c *Client) RegisterSelfMapping(clientIP string, mapping map[string]string, replace bool) error {
	return c.internalSetValues(SELF_MAPPING_PATH, clientIP, mapping, replace)
}

func (c *Client) UnregisterSelfMapping(clientIP string) error {
	return c.internalDelete(SELF_MAPPING_PATH, clientIP, true)
}
