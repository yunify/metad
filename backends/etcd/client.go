package etcd

import (
	"crypto/tls"
	"crypto/x509"
	"github.com/coreos/etcd/client"
	"github.com/yunify/metadata-proxy/log"
	"github.com/yunify/metadata-proxy/store"
	"github.com/yunify/metadata-proxy/util"
	"golang.org/x/net/context"
	"io/ioutil"
	"net"
	"net/http"
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
func (c *Client) GetValues(key string) (map[string]string, error) {
	return c.internalGetValues(c.prefix, key)
}

func (c *Client) internalGetValues(prefix, key string) (map[string]string, error) {
	vars := make(map[string]string)
	resp, err := c.client.Get(context.Background(), util.AppendPathPrefix(key, prefix), &client.GetOptions{
		Recursive: true,
		Sort:      true,
		Quorum:    true,
	})
	if err != nil {
		return nil, err
	}
	err = nodeWalk(prefix, resp.Node, vars)
	if err != nil {
		return vars, err
	}
	return vars, nil
}

// nodeWalk recursively descends nodes, updating vars.
func nodeWalk(prefix string, node *client.Node, vars map[string]string) error {
	if node != nil {
		key := node.Key
		if !node.Dir {
			key = util.TrimPathPrefix(key, prefix)
			vars[key] = node.Value
		} else {
			for _, node := range node.Nodes {
				nodeWalk(prefix, node, vars)
			}
		}
	}
	return nil
}

func (c *Client) internalSync(prefix string, store store.Store, stopChan chan bool) {
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

		if !inited {
			val, err := c.internalGetValues(prefix, "/")
			if err != nil {
				log.Error("GetValue from etcd key:%s error: %s", prefix, err.Error())
				time.Sleep(time.Duration(1000) * time.Millisecond)
				continue
			}
			store.SetBulk(val)
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
	//TODO wait etcd 3.1.0 support watch children dir action.
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

func (c *Client) SetValues(values map[string]string) error {
	return c.internalSetValue(c.prefix, values)
}

func (c *Client) internalSetValue(prefix string, values map[string]string) error {
	for k, v := range values {
		k = util.AppendPathPrefix(k, prefix)
		_, err := c.client.Set(context.Background(), k, v, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) Delete(key string) error {
	return c.internalDelete(c.prefix, key)
}

func (c *Client) internalDelete(prefix, key string) error {
	key = util.AppendPathPrefix(key, prefix)
	log.Debug("Delete from backend, key:%s", key)
	_, err := c.client.Delete(context.Background(), key, &client.DeleteOptions{Recursive: true})
	return err
}

func (c *Client) SyncSelfMapping(mapping store.Store, stopChan chan bool) {
	go c.internalSync(SELF_MAPPING_PATH, mapping, stopChan)
}

func (c *Client) RegisterSelfMapping(clientIP string, mapping map[string]string) error {
	prefix := util.AppendPathPrefix(clientIP, SELF_MAPPING_PATH)
	oldMapping, _ := c.internalGetValues(prefix, "/")
	if oldMapping != nil {
		for k, _ := range oldMapping {
			if _, ok := mapping[k]; !ok {
				c.internalDelete(prefix, k)
			}
		}
	}
	return c.internalSetValue(prefix, mapping)
}

func (c *Client) UnregisterSelfMapping(clientIP string) error {
	return c.internalDelete(SELF_MAPPING_PATH, clientIP)
}
