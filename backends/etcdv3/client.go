package etcdv3

import (
	"crypto/tls"
	"crypto/x509"
	client "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/yunify/metadata-proxy/log"
	"github.com/yunify/metadata-proxy/store"
	"github.com/yunify/metadata-proxy/util"
	"github.com/yunify/metadata-proxy/util/flatmap"
	"golang.org/x/net/context"
	"io/ioutil"
	"reflect"
	"time"
)

const SELF_MAPPING_PATH = "/_metadata-proxy/mapping"

// Client is a wrapper around the etcd client
type Client struct {
	client *client.Client
	prefix string
}

// NewEtcdClient returns an *etcd.Client with a connection to named machines.
func NewEtcdClient(prefix string, machines []string, cert, key, caCert string, basicAuth bool, username string, password string) (*Client, error) {
	var c *client.Client
	var err error

	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
	}

	cfg := client.Config{
		Endpoints:   machines,
		DialTimeout: time.Duration(3) * time.Second,
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

	cfg.TLS = tlsConfig

	c, err = client.New(cfg)
	if err != nil {
		return nil, err
	}

	return &Client{c, prefix}, nil
}

// GetValues queries etcd for key prefix.
func (c *Client) GetValues(key string) (map[string]interface{}, error) {
	m, err := c.internalGetValues(c.prefix, key)
	if err != nil {
		return nil, err
	}
	return flatmap.Expand(m, util.AppendPathPrefix(key, c.prefix)), nil
}

func (c *Client) internalGetValues(prefix, key string) (map[string]string, error) {
	vars := make(map[string]string)
	resp, err := c.client.Get(context.Background(), util.AppendPathPrefix(key, prefix), client.WithPrefix())
	if err != nil {
		return nil, err
	}

	err = handleGetResp(prefix, resp, vars)
	if err != nil {
		return vars, err
	}
	return vars, nil
}

// GetValue queries etcd for key
func (c *Client) GetValue(key string) (string, error) {
	return c.internalGetValue(c.prefix, key)
}

func (c *Client) internalGetValue(prefix, key string) (string, error) {
	resp, err := c.client.Get(context.Background(), util.AppendPathPrefix(key, prefix))
	if err != nil {
		return "", err
	}
	if len(resp.Kvs) == 0 {
		return "", nil
	} else {
		return string(resp.Kvs[0].Value), nil
	}
}

// nodeWalk recursively descends nodes, updating vars.
func handleGetResp(prefix string, resp *client.GetResponse, vars map[string]string) error {
	if resp != nil {
		kvs := resp.Kvs
		for _, kv := range kvs {
			vars[string(kv.Key)] = string(kv.Value)
		}
		//TODO handle resp.More
	}
	return nil
}

func (c *Client) internalSync(prefix string, store store.Store, stopChan chan bool) {

	defer func() {
		if r := recover(); r != nil {
			log.Error("Sync Recover: %v, try restart.", r)
			time.Sleep(time.Duration(1000) * time.Millisecond)
			c.internalSync(prefix, store, stopChan)
		}
	}()

	var rev int64 = 0
	inited := false
	for {
		ctx, cancel := context.WithCancel(context.Background())
		watchChan := c.client.Watch(ctx, prefix, client.WithPrefix(), client.WithRev(rev))

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
				time.Sleep(time.Duration(1000) * time.Millisecond)
				continue
			}
			store.Sets("/", val)
			inited = true
		}
		for resp := range watchChan {
			processSyncChange(prefix, store, &resp)
			rev = resp.Header.Revision
		}
	}
}

func processSyncChange(prefix string, store store.Store, resp *client.WatchResponse) {
	for _, event := range resp.Events {
		key := util.TrimPathPrefix(string(event.Kv.Key), prefix)
		value := string(event.Kv.Value)
		log.Debug("process sync change, event_type: %s, prefix: %v, key:%v, value: %v ", event.Type, prefix, key, value)
		switch event.Type {
		case mvccpb.PUT:
			store.Set(key, false, value)
		case mvccpb.DELETE:
			store.Delete(key)
		default:
			log.Warning("Unknow watch event type: %s ", event.Type)
			store.Set(key, false, value)

		}
	}
}

func (c *Client) Sync(store store.Store, stopChan chan bool) {
	go c.internalSync(c.prefix, store, stopChan)
}

func (c *Client) SetValues(key string, values map[string]interface{}, replace bool) error {
	flatValues := flatmap.Flatten(values)
	return c.internalSetValues(c.prefix, key, flatValues, replace)
}

func (c *Client) internalSetValues(prefix string, key string, values map[string]string, replace bool) error {
	txn := c.client.Txn(context.TODO())

	new_prefix := util.AppendPathPrefix(key, prefix)
	ops := make([]client.Op, 0, len(values)+1)
	if replace {
		//delete and put can not in same txn.
		c.internalDelete(prefix, key, true)
	}
	for k, v := range values {
		k = util.AppendPathPrefix(k, new_prefix)
		ops = append(ops, client.OpPut(k, v))
		log.Debug("SetValue prefix:%s, key:%s, value:%s", new_prefix, k, v)
	}
	txn.Then(ops...)
	resp, err := txn.Commit()
	log.Debug("SetValues err:%v, resp:%v", err, resp)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) SetValue(key string, value string) error {
	return c.internalSetValue(c.prefix, key, value)
}

func (c *Client) internalSetValue(prefix string, key string, value string) error {
	key = util.AppendPathPrefix(key, prefix)
	resp, err := c.client.Put(context.TODO(), key, value)
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
	var err error
	if dir {
		if key[len(key)-1] != '/' {
			key = key + "/"
		}
		_, err = c.client.Delete(context.Background(), key, client.WithPrefix())
	} else {
		_, err = c.client.Delete(context.Background(), key)
	}
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
