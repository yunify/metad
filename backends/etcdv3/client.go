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

// GetValues queries etcd for nodePath prefix.
func (c *Client) GetValues(nodePath string) (map[string]interface{}, error) {
	m, err := c.internalGetValues(c.prefix, nodePath)
	if err != nil {
		return nil, err
	}
	return flatmap.Expand(m, nodePath), nil
}

func (c *Client) internalGetValues(prefix, nodePath string) (map[string]string, error) {
	vars := make(map[string]string)
	resp, err := c.client.Get(context.Background(), util.AppendPathPrefix(nodePath, prefix), client.WithPrefix())
	if err != nil {
		return nil, err
	}

	err = handleGetResp(prefix, resp, vars)
	if err != nil {
		return nil, err
	}
	log.Debug("GetValues prefix:%s, nodePath:%s, resp:%v", prefix, nodePath, vars)
	return vars, nil
}

// GetValue queries etcd for nodePath
func (c *Client) GetValue(nodePath string) (string, error) {
	return c.internalGetValue(c.prefix, nodePath)
}

func (c *Client) internalGetValue(prefix, nodePath string) (string, error) {
	resp, err := c.client.Get(context.Background(), util.AppendPathPrefix(nodePath, prefix))
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
				log.Error("GetValue from etcd nodePath:%s, error-type: %s, error: %s", prefix, reflect.TypeOf(err), err.Error())
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
		nodePath := string(event.Kv.Key)
		nodePath = util.TrimPathPrefix(nodePath, prefix)
		value := string(event.Kv.Value)
		log.Debug("process sync change, event_type: %s, prefix: %v, nodePath:%v, value: %v ", event.Type, prefix, nodePath, value)
		switch event.Type {
		case mvccpb.PUT:
			store.Set(nodePath, false, value)
		case mvccpb.DELETE:
			store.Delete(nodePath)
		default:
			log.Warning("Unknow watch event type: %s ", event.Type)
			store.Set(nodePath, false, value)

		}
	}
}

func (c *Client) Sync(store store.Store, stopChan chan bool) {
	go c.internalSync(c.prefix, store, stopChan)
}

func (c *Client) Set(nodePath string, value interface{}, replace bool) error {
	return c.internalSet(c.prefix, nodePath, value, replace)
}

func (c *Client) internalSet(prefix, nodePath string, value interface{}, replace bool) error {
	switch t := value.(type) {
	case map[string]interface{}, map[string]string, []interface{}:
		flatValues := flatmap.Flatten(t)
		return c.internalSetValues(prefix, nodePath, flatValues, replace)
	case string:
		return c.internalSetValue(prefix, nodePath, t)
	default:
		log.Warning("Set unexpect value type: %s", reflect.TypeOf(value))
		val := fmt.Sprintf("%v", t)
		return c.internalSetValue(prefix, nodePath, val)
	}
}

func (c *Client) SetValues(nodePath string, values map[string]interface{}, replace bool) error {
	flatValues := flatmap.Flatten(values)
	return c.internalSetValues(c.prefix, nodePath, flatValues, replace)
}

func (c *Client) internalSetValues(prefix string, nodePath string, values map[string]string, replace bool) error {

	new_prefix := util.AppendPathPrefix(nodePath, prefix)
	ops := make([]client.Op, 0, len(values)+1)
	if replace {
		//delete and put can not in same txn.
		c.internalDelete(prefix, nodePath, true)
	}
	for k, v := range values {
		k = util.AppendPathPrefix(k, new_prefix)
		ops = append(ops, client.OpPut(k, v))
		log.Debug("SetValue prefix:%s, nodePath:%s, value:%s", new_prefix, k, v)
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

func (c *Client) SetValue(nodePath string, value string) error {
	return c.internalSetValue(c.prefix, nodePath, value)
}

func (c *Client) internalSetValue(prefix string, nodePath string, value string) error {
	nodePath = util.AppendPathPrefix(nodePath, prefix)
	resp, err := c.client.Put(context.TODO(), nodePath, value)
	log.Debug("SetValue nodePath: %s, value:%s, resp:%v", nodePath, value, resp)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) Delete(nodePath string, dir bool) error {
	return c.internalDelete(c.prefix, nodePath, dir)
}

func (c *Client) internalDelete(prefix, nodePath string, dir bool) error {
	log.Debug("Delete from backend, prefix:%s, nodePath:%s, dir:%v", prefix, nodePath, dir)
	nodePath = util.AppendPathPrefix(nodePath, prefix)
	var err error
	if dir {
		if nodePath[len(nodePath)-1] != '/' {
			nodePath = nodePath + "/"
		}
		_, err = c.client.Delete(context.Background(), nodePath, client.WithPrefix())
	} else {
		_, err = c.client.Delete(context.Background(), nodePath)
	}
	return err
}

func (c *Client) SyncMapping(mapping store.Store, stopChan chan bool) {
	go c.internalSync(SELF_MAPPING_PATH, mapping, stopChan)
}

func (c *Client) UpdateMapping(nodePath string, mapping interface{}, replace bool) error {
	log.Debug("UpdateMapping nodePath:%s, mapping:%v, replace:%v", nodePath, mapping, replace)
	return c.internalSet(SELF_MAPPING_PATH, nodePath, mapping, replace)
}

func (c *Client) DeleteMapping(nodePath string) error {
	nodePath = path.Join("/", nodePath)
	// mapping nodePath path only two level /$ip/$key, split: [,ip,key]
	dir := len(strings.Split(nodePath, "/")) <= 2
	return c.internalDelete(SELF_MAPPING_PATH, nodePath, dir)
}
