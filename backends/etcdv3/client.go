package etcdv3

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	client "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/yunify/metadata-proxy/log"
	"github.com/yunify/metadata-proxy/store"
	"github.com/yunify/metadata-proxy/util"
	"github.com/yunify/metadata-proxy/util/flatmap"
	"golang.org/x/net/context"
	"io/ioutil"
	"path"
	"reflect"
	"strings"
	"time"
)

const SELF_MAPPING_PATH = "/_metadata-proxy/mapping"

var (
	//see github.com/coreos/etcd/etcdserver/api/v3rpc/key.go
	MaxOpsPerTxn = 128
)

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
	return flatmap.Expand(m, key), nil
}

func (c *Client) internalGetValues(prefix, key string) (map[string]string, error) {
	vars := make(map[string]string)
	resp, err := c.client.Get(context.Background(), util.AppendPathPrefix(key, prefix), client.WithPrefix())
	if err != nil {
		return nil, err
	}

	err = handleGetResp(prefix, resp, vars)
	if err != nil {
		return nil, err
	}
	log.Debug("GetValues prefix:%s, key:%s, resp:%v", prefix, key, vars)
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
			vars[util.TrimPathPrefix(string(kv.Key), prefix)] = string(kv.Value)
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
			val, err := c.internalGetValues(prefix, "/")
			if err != nil {
				log.Error("GetValue from etcd key:%s, error-type: %s, error: %s", prefix, reflect.TypeOf(err), err.Error())
				time.Sleep(time.Duration(1000) * time.Millisecond)
				continue
			}
			store.SetBulk("/", val)
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

func (c *Client) Set(key string, value interface{}, replace bool) error {
	return c.internalSet(c.prefix, key, value, replace)
}

func (c *Client) internalSet(prefix, key string, value interface{}, replace bool) error {
	switch t := value.(type) {
	case map[string]interface{}, map[string]string, []interface{}:
		flatValues := flatmap.Flatten(t)
		return c.internalSetValues(prefix, key, flatValues, replace)
	case string:
		return c.internalSetValue(prefix, key, t)
	default:
		log.Warning("Set unexpect value type: %s", reflect.TypeOf(value))
		val := fmt.Sprintf("%v", t)
		return c.internalSetValue(prefix, key, val)
	}
}

func (c *Client) SetValues(key string, values map[string]interface{}, replace bool) error {
	flatValues := flatmap.Flatten(values)
	return c.internalSetValues(c.prefix, key, flatValues, replace)
}

func (c *Client) internalSetValues(prefix string, key string, values map[string]string, replace bool) error {

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
	for ok := true; ok; {
		var commitOps []client.Op
		if len(ops) > MaxOpsPerTxn {
			commitOps = ops[:MaxOpsPerTxn]
			ops = ops[MaxOpsPerTxn:]
		} else {
			commitOps = ops
			ok = false
		}
		txn := c.client.Txn(context.TODO())
		txn.Then(commitOps...)
		resp, err := txn.Commit()
		log.Debug("SetValues err:%v, resp:%v", err, resp)
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
	log.Debug("Delete from backend, prefix:%s, key:%s, dir:%v", prefix, key, dir)
	key = util.AppendPathPrefix(key, prefix)
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

func (c *Client) SyncMapping(mapping store.Store, stopChan chan bool) {
	go c.internalSync(SELF_MAPPING_PATH, mapping, stopChan)
}

func (c *Client) UpdateMapping(key string, mapping interface{}, replace bool) error {
	log.Debug("UpdateMapping key:%s, mapping:%v, replace:%v", key, mapping, replace)
	return c.internalSet(SELF_MAPPING_PATH, key, mapping, replace)
}

func (c *Client) DeleteMapping(key string) error {
	key = path.Join("/", key)
	// mapping key path only two level /$ip/$key, split: [,ip,key]
	dir := len(strings.Split(key, "/")) <= 2
	return c.internalDelete(SELF_MAPPING_PATH, key, dir)
}
