package store

const (
	Update = "update"
	Delete = "delete"
)

type Event struct {
	Action string `json:"action"`
	Path   string `json:"path"`
}

func newEvent(action string, path string) *Event {
	return &Event{
		Action: action,
		Path:   path,
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

func newWatcher(node *node) *watcher {
	w := &watcher{
		eventChan: make(chan *Event, 10),
		node:      node,
	}
	return w
}

func (w *watcher) EventChan() chan *Event {
	return w.eventChan
}

func (w *watcher) Remove() {
	w.node.store.watcherLock.Lock()
	defer w.node.store.watcherLock.Unlock()

	close(w.eventChan)
	if w.remove != nil {
		w.remove()
	}
}
