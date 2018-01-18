// Copyright 2018 Yunify Inc. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

package local

import (
	"github.com/yunify/metad/log"
	"github.com/yunify/metad/store"
)

// a backend just for test.
type Client struct {
	data        store.Store
	mapping     store.Store
	rules       map[string][]store.AccessRule
	accessStore store.AccessStore
}

func NewLocalClient() (*Client, error) {
	return &Client{
		data:    store.New(),
		mapping: store.New(),
		rules:   map[string][]store.AccessRule{},
	}, nil
}

// Get queries etcd for nodePath.
func (c *Client) Get(nodePath string, dir bool) (interface{}, error) {
	_, r := c.data.Get(nodePath)
	if r != nil {
		return r, nil
	} else {
		if dir {
			return map[string]interface{}{}, nil
		} else {
			return "", nil
		}
	}
}

func (c *Client) Put(nodePath string, value interface{}, replace bool) error {
	if replace {
		c.data.Delete(nodePath)
	}
	c.data.Put(nodePath, value)
	return nil
}

func (c *Client) Delete(nodePath string, dir bool) error {
	c.data.Delete(nodePath)
	return nil
}

func (c *Client) Sync(s store.Store, stopChan chan bool) {
	go c.internalSync("data", c.data, s, stopChan)
}

func (c *Client) GetMapping(nodePath string, dir bool) (interface{}, error) {
	_, r := c.mapping.Get(nodePath)
	if r != nil {
		return r, nil
	} else {
		if dir {
			return map[string]interface{}{}, nil
		} else {
			return "", nil
		}
	}
}

func (c *Client) PutMapping(nodePath string, mapping interface{}, replace bool) error {
	if replace {
		c.mapping.Delete(nodePath)
	}
	c.mapping.Put(nodePath, mapping)
	return nil
}

func (c *Client) DeleteMapping(nodePath string, dir bool) error {
	c.mapping.Delete(nodePath)
	return nil
}

func (c *Client) SyncMapping(mapping store.Store, stopChan chan bool) {
	go c.internalSync("mapping", c.mapping, mapping, stopChan)
}

func (c *Client) GetAccessRule() (map[string][]store.AccessRule, error) {
	result := make(map[string][]store.AccessRule, len(c.rules))
	for k, v := range c.rules {
		result[k] = v
	}
	return result, nil
}

func (c *Client) PutAccessRule(rules map[string][]store.AccessRule) error {
	for k, v := range rules {
		c.rules[k] = v
		if c.accessStore != nil {
			c.accessStore.Put(k, v)
		}
	}
	return nil
}

func (c *Client) DeleteAccessRule(hosts []string) error {
	for _, host := range hosts {
		delete(c.rules, host)
		if c.accessStore != nil {
			c.accessStore.Delete(host)
		}
	}
	return nil
}

func (c *Client) SyncAccessRule(accessStore store.AccessStore, stopChan chan bool) {
	c.accessStore = accessStore
	for k, v := range c.rules {
		c.accessStore.Put(k, v)
	}
	go func() {
		select {
		case <-stopChan:
			c.accessStore = nil
		}
	}()
}

func (c *Client) internalSync(name string, from store.Store, to store.Store, stopChan chan bool) {
	w := from.Watch("/", 5000)
	_, meta := from.Get("/")
	if meta != nil {
		to.Put("/", meta)
	}
	for {
		select {
		case e, ok := <-w.EventChan():
			if !ok {
				return
			}
			log.Debug("processEvent %s %s %s", e.Action, e.Path, e.Value)
			switch e.Action {
			case store.Delete:
				to.Delete(e.Path)
			case store.Update:
				to.Put(e.Path, e.Value)
			}
		case <-stopChan:
			log.Info("Stop sync %s", name)
			w.Remove()
		}
	}
}
