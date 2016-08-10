package etcd

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/client"
	"github.com/yunify/metadata-proxy/store"
	"golang.org/x/net/context"
)

// Client is a wrapper around the etcd client
type Client struct {
	client client.KeysAPI
}

// NewEtcdClient returns an *etcd.Client with a connection to named machines.
func NewEtcdClient(machines []string, cert, key, caCert string, basicAuth bool, username string, password string) (*Client, error) {
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
			return &Client{kapi}, err
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
			return &Client{kapi}, err
		}
		tlsConfig.Certificates = []tls.Certificate{tlsCert}
	}

	transport.TLSClientConfig = tlsConfig
	cfg.Transport = transport

	c, err = client.New(cfg)
	if err != nil {
		return &Client{kapi}, err
	}

	kapi = client.NewKeysAPI(c)
	return &Client{kapi}, nil
}

// GetValues queries etcd for key Recursive:true.
func (c *Client) GetValues(key string) (map[string]string, error) {
	vars := make(map[string]string)
	resp, err := c.client.Get(context.Background(), key, &client.GetOptions{
		Recursive: true,
		Sort:      true,
		Quorum:    true,
	})
	if err != nil {
		return vars, err
	}
	err = nodeWalk(resp.Node, vars)
	if err != nil {
		return vars, err
	}
	return vars, nil
}

// nodeWalk recursively descends nodes, updating vars.
func nodeWalk(node *client.Node, vars map[string]string) error {
	if node != nil {
		key := node.Key
		if !node.Dir {
			vars[key] = node.Value
		} else {
			for _, node := range node.Nodes {
				nodeWalk(node, vars)
			}
		}
	}
	return nil
}

func (c *Client) internalSync(key string, store store.Store, stopChan chan bool) {
	var waitIndex uint64 = 0
	inited := false
	for {
		watcher := c.client.Watcher(key, &client.WatcherOptions{AfterIndex: waitIndex, Recursive: true})
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
			val, err := c.GetValues(key)
			if err != nil {
				log.Errorf("GetValue from etcd key:%s error: %s", key, err.Error())
				time.Sleep(time.Duration(1000) * time.Millisecond)
				continue
			}
			store.SetBulk(val)
			inited = true
		}

		resp, err := watcher.Next(ctx)
		if err != nil {
			log.Errorf("Watch etcd error: %s", err.Error())
			time.Sleep(time.Duration(1000) * time.Millisecond)
			continue
		}

		vars := make(map[string]string)

		err = nodeWalk(resp.Node, vars)
		if err != nil {
			log.Errorf("Watch etcd walk error: %s", err.Error())
			time.Sleep(time.Duration(1000) * time.Millisecond)
			continue
		}
		store.SetBulk(vars)
		waitIndex = resp.Node.ModifiedIndex
	}
}

func (c *Client) Sync(key string, store store.Store, stopChan chan bool) {
	go c.internalSync(key, store, stopChan)
}

func (c *Client) SetValues(values map[string]string) error {
	for k, v := range values {
		_, err := c.client.Set(context.Background(), k, v, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) Delete(key string) error {
	_, err := c.client.Delete(context.Background(), key, &client.DeleteOptions{Recursive: true})
	return err
}
