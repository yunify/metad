// Copyright 2018 Yunify Inc. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

package store

import (
	"fmt"
	"path"
	"sync"
)

const (
	Update = "UPDATE"
	Delete = "DELETE"
)

type Event struct {
	Action string `json:"action"`
	Path   string `json:"path"`
	Value  string `json:"value"`
}

func (e *Event) String() string {
	return fmt.Sprintf("%s:%s|%s", e.Path, e.Action, e.Value)
}

func newEvent(action string, path string, value string) *Event {
	return &Event{
		Action: action,
		Path:   path,
		Value:  value,
	}
}

type Watcher interface {
	EventChan() chan *Event
	Remove()
}

type watcher struct {
	eventChan chan *Event
	removed   bool
	node      *node
	remove    func()
}

func newWatcher(node *node, bufLen int) *watcher {
	w := &watcher{
		eventChan: make(chan *Event, bufLen),
		node:      node,
	}
	return w
}

func (w *watcher) EventChan() chan *Event {
	return w.eventChan
}

func (w *watcher) Remove() {
	w.node.watcherLock.Lock()
	defer w.node.watcherLock.Unlock()

	close(w.eventChan)
	if w.remove != nil {
		w.remove()
	}
}

type aggregateWatcher struct {
	watchers  map[string]Watcher
	eventChan chan *Event
	closeWait *sync.WaitGroup
}

func NewAggregateWatcher(watchers map[string]Watcher) Watcher {
	eventChan := make(chan *Event, len(watchers)*50)
	waitGroup := &sync.WaitGroup{}
	waitGroup.Add(len(watchers))
	for pathPrefix, watcher := range watchers {
		go func(pathPrefix string, watcher Watcher) {
			for {
				select {
				case event, ok := <-watcher.EventChan():
					if ok {
						eventChan <- newEvent(event.Action, path.Join(pathPrefix, event.Path), event.Value)
					} else {
						waitGroup.Done()
						return
					}
				}
			}
		}(pathPrefix, watcher)
	}
	return &aggregateWatcher{watchers: watchers, eventChan: eventChan, closeWait: waitGroup}
}

func (w *aggregateWatcher) EventChan() chan *Event {
	return w.eventChan
}

func (w *aggregateWatcher) Remove() {
	for _, watcher := range w.watchers {
		watcher.Remove()
	}
	//wait all sub watcher's go routine exit.
	w.closeWait.Wait()
	close(w.eventChan)
}
