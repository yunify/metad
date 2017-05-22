package local

import (
	"context"
	"github.com/yunify/metad/log"
	"github.com/yunify/metad/store"
)

// a backend just for test.
type Client struct {
	data    store.Store
	mapping store.Store
}

func NewLocalClient() (*Client, error) {
	return &Client{
		data:    store.New(),
		mapping: store.New(),
	}, nil
}

func newDefaultContext() context.Context {
	ctx := store.WithVisibility(nil, store.VisibilityLevelPrivate)
	return ctx
}

// Get queries etcd for nodePath.
func (c *Client) Get(nodePath string, dir bool) (interface{}, error) {
	ctx := newDefaultContext()
	_, r := c.data.Get(ctx, nodePath)
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
	ctx := newDefaultContext()
	if replace {
		c.data.Delete(ctx, nodePath)
	}
	c.data.Put(ctx, nodePath, value)
	return nil
}

func (c *Client) Delete(nodePath string, dir bool) error {
	ctx := newDefaultContext()
	c.data.Delete(ctx, nodePath)
	return nil
}

func (c *Client) Sync(s store.Store, stopChan chan bool) {
	go c.internalSync("data", c.data, s, stopChan)
}

func (c *Client) GetMapping(nodePath string, dir bool) (interface{}, error) {
	ctx := newDefaultContext()
	_, r := c.mapping.Get(ctx, nodePath)
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
	ctx := newDefaultContext()
	if replace {
		c.mapping.Delete(ctx, nodePath)
	}
	c.mapping.Put(ctx, nodePath, mapping)
	return nil
}

func (c *Client) DeleteMapping(nodePath string, dir bool) error {
	ctx := newDefaultContext()
	c.mapping.Delete(ctx, nodePath)
	return nil
}

func (c *Client) SyncMapping(mapping store.Store, stopChan chan bool) {
	go c.internalSync("mapping", c.mapping, mapping, stopChan)
}

func (c *Client) internalSync(name string, from store.Store, to store.Store, stopChan chan bool) {
	ctx := newDefaultContext()
	w := from.Watch(ctx, "/", 5000)
	_, meta := from.Get(ctx, "/")
	if meta != nil {
		to.Put(ctx, "/", meta)
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
				to.Delete(ctx, e.Path)
			case store.Update:
				to.Put(ctx, e.Path, e.Value)
			}
		case <-stopChan:
			log.Info("Stop sync %s", name)
			w.Remove()
		}
	}
}
