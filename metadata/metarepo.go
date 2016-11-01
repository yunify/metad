package metadata

import (
	"errors"
	"fmt"
	"github.com/yunify/metad/backends"
	"github.com/yunify/metad/log"
	"github.com/yunify/metad/store"
	"github.com/yunify/metad/util"
	"github.com/yunify/metad/util/flatmap"
	"net"
	"path"
	"reflect"
	"strings"
	"time"
)

type MetadataRepo struct {
	onlySelf        bool
	mapping         store.Store
	storeClient     backends.StoreClient
	data            store.Store
	metaStopChan    chan bool
	mappingStopChan chan bool
	timerPool       *util.TimerPool
}

func New(onlySelf bool, storeClient backends.StoreClient) *MetadataRepo {
	metadataRepo := MetadataRepo{
		onlySelf:        onlySelf,
		mapping:         store.New(),
		storeClient:     storeClient,
		data:            store.New(),
		metaStopChan:    make(chan bool),
		mappingStopChan: make(chan bool),
		timerPool:       util.NewTimerPool(100 * time.Millisecond),
	}
	return &metadataRepo
}

func (r *MetadataRepo) SetOnlySelf(onlySelf bool) {
	r.onlySelf = onlySelf
}

func (r *MetadataRepo) StartSync() {
	log.Info("Start Sync")
	r.startMetaSync()
	r.startMappingSync()
}

func (r *MetadataRepo) startMetaSync() {
	r.storeClient.Sync(r.data, r.metaStopChan)
}

func (r *MetadataRepo) startMappingSync() {
	r.storeClient.SyncMapping(r.mapping, r.mappingStopChan)
}

func (r *MetadataRepo) ReSync() {
	log.Info("ReSync")
	//TODO lock
	r.StopSync()
	r.data.Delete("/")
	r.mapping.Delete("/")
	r.StartSync()
}

func (r *MetadataRepo) StopSync() {
	log.Info("Stop Sync")
	r.metaStopChan <- true
	r.mappingStopChan <- true
}

func (r *MetadataRepo) Root(clientIP string, nodePath string) (currentVersion int64, val interface{}) {
	nodePath = path.Join("/", nodePath)
	if r.onlySelf {
		currentVersion = r.data.Version()
		if nodePath == "/" {
			mapVal := make(map[string]interface{})
			selfVal := r.Self(clientIP, "/")
			if selfVal != nil {
				mapVal["self"] = selfVal
			}
			val = mapVal
		}
	} else {
		currentVersion, val = r.data.Get(nodePath)
		if val != nil && nodePath == "/" {
			selfVal := r.Self(clientIP, "/")
			if selfVal != nil {
				mapVal, ok := val.(map[string]interface{})
				if ok {
					mapVal["self"] = selfVal
				}
			}
		}
	}
	return
}

func (r *MetadataRepo) Watch(clientIP string, nodePath string, closeChan <-chan bool) interface{} {
	nodePath = path.Join("/", nodePath)

	if r.onlySelf {
		if nodePath == "/" {
			return r.WatchSelf(clientIP, "/", closeChan)
		} else {
			return nil
		}
	} else {
		w := r.data.Watch(nodePath)
		return r.changeToResult(w, closeChan)
	}
}

var TIMER_NIL *time.Timer = &time.Timer{C: nil}

func (r *MetadataRepo) changeToResult(watcher store.Watcher, stopChan <-chan bool) interface{} {
	defer watcher.Remove()
	m := make(map[string]string)
	timer := TIMER_NIL

	for {
		var finish bool = false
		select {
		case e, ok := <-watcher.EventChan():
			if ok {
				value := fmt.Sprintf("%s|%s", e.Action, e.Value)
				// if event is one leaf node, just return value.
				if e.Path == "/" {
					return value
				}
				m[e.Path] = value
				timer = r.timerPool.AcquireTimer()
			} else {
				finish = true
			}
		case <-timer.C:
			finish = true
		case <-stopChan:
			//when stop, return empty map, discard prev result.
			m = make(map[string]string)
			finish = true
		}
		if finish {
			if timer.C != nil {
				r.timerPool.ReleaseTimer(timer)
			}
			break
		}
		//TODO check map size, avoid too big result.
	}
	return flatmap.Expand(m, "/")
}

func (r *MetadataRepo) WatchSelf(clientIP string, nodePath string, closeChan <-chan bool) interface{} {
	nodePath = path.Join(clientIP, "/", nodePath)
	if log.IsDebugEnable() {
		log.Debug("WatchSelf nodePath: %s", nodePath)
	}
	mappingData := r.GetMapping(nodePath)
	if mappingData == nil {
		return nil
	}
	mappingWatcher := r.mapping.Watch(nodePath)
	defer mappingWatcher.Remove()

	stopChan := make(chan bool)
	defer close(stopChan)
	go func() {
		select {
		case _, ok := <-mappingWatcher.EventChan():
			if ok {
				stopChan <- true
			}
		case <-closeChan:
			stopChan <- true
		case <-stopChan:
			return
		}
	}()

	mapping, mok := mappingData.(map[string]interface{})
	if !mok {
		dataNodePath := fmt.Sprintf("%s", mappingData)
		//log.Debug("watcher: %v", dataNodePath)
		w := r.data.Watch(dataNodePath)
		return r.changeToResult(w, stopChan)
	} else {
		flatMapping := flatmap.Flatten(mapping)
		watchers := make(map[string]store.Watcher)
		for k, v := range flatMapping {
			watchers[k] = r.data.Watch(v)
		}
		//log.Debug("aggWatcher: %v", watchers)
		aggWatcher := store.NewAggregateWatcher(watchers)
		return r.changeToResult(aggWatcher, stopChan)
	}
}

func (r *MetadataRepo) Self(clientIP string, nodePath string) interface{} {
	if clientIP == "" {
		panic(errors.New("clientIP must not be empty."))
	}
	nodePath = path.Join("/", nodePath)
	mappingData := r.GetMapping(path.Join("/", clientIP))
	if mappingData == nil {
		if log.IsDebugEnable() {
			log.Debug("Can not find mapping for %s", clientIP)
		}
		return nil
	}
	mapping, mok := mappingData.(map[string]interface{})
	if !mok {
		log.Warning("Mapping for %s is not a map, result:%v", clientIP, mappingData)
		return nil
	}
	return r.getMappingDatas(nodePath, mapping)
}

func (r *MetadataRepo) getMappingData(nodePath, link string) interface{} {
	nodePath = path.Join(link, nodePath)
	_, val := r.data.Get(nodePath)
	return val
}

func (r *MetadataRepo) getMappingDatas(nodePath string, mapping map[string]interface{}) interface{} {
	nodePath = path.Join("/", nodePath)
	paths := strings.Split(nodePath, "/")[1:] // trim first blank item
	// nodePath is "/"
	if paths[0] == "" {
		meta := make(map[string]interface{})
		for k, v := range mapping {
			submapping, isMap := v.(map[string]interface{})
			if isMap {
				val := r.getMappingDatas("/", submapping)
				if val != nil {
					meta[k] = val
				} else {
					log.Warning("Can not get values from backend by mapping: %v", submapping)
				}
			} else {
				subNodePath := fmt.Sprintf("%v", v)
				val := r.getMappingData("/", subNodePath)
				if val != nil {
					meta[k] = val
				} else {
					log.Warning("Can not get values from backend by mapping: %v", subNodePath)
				}
			}

		}
		return meta
	} else {
		elemName := paths[0]
		elemValue, ok := mapping[elemName]
		if ok {
			submapping, isMap := elemValue.(map[string]interface{})
			if isMap {
				return r.getMappingDatas(path.Join(paths[1:]...), submapping)
			} else {
				return r.getMappingData(path.Join(paths[1:]...), fmt.Sprintf("%v", elemValue))
			}
		} else {
			if log.IsDebugEnable() {
				log.Debug("Can not find mapping for : %v, mapping:%v", nodePath, mapping)
			}
			return nil
		}
	}
}

func (r *MetadataRepo) GetData(nodePath string) interface{} {
	_, val := r.data.Get(nodePath)
	return val
}

func (r *MetadataRepo) PutData(nodePath string, data interface{}, replace bool) error {
	return r.storeClient.Put(nodePath, data, replace)
}

func (r *MetadataRepo) DeleteData(nodePath string, subs ...string) error {
	err := checkSubs(subs)
	if err != nil {
		return err
	}
	if len(subs) > 0 {
		for _, sub := range subs {
			subPath := path.Join(nodePath, sub)
			_, v := r.data.Get(subPath)
			// if subPath metadata not exist, just ignore.
			if v != nil {
				_, dir := v.(map[string]interface{})
				err = r.storeClient.Delete(subPath, dir)
				if err != nil {
					return err
				}
			}
		}
		return nil
	} else {
		_, v := r.data.Get(nodePath)
		if v != nil {
			_, dir := v.(map[string]interface{})
			return r.storeClient.Delete(nodePath, dir)
		}
		return nil
	}

}

func (r *MetadataRepo) GetMapping(nodePath string) interface{} {
	_, val := r.mapping.Get(nodePath)
	return val
}

func (r *MetadataRepo) PutMapping(nodePath string, data interface{}, replace bool) error {
	nodePath = path.Join("/", nodePath)
	if nodePath == "/" {
		m, ok := data.(map[string]interface{})
		if !ok {
			log.Warning("Unexpect data type for mapping: %s", reflect.TypeOf(data))
			return errors.New("mapping data should be json object.")
		}
		for k, v := range m {
			ip := net.ParseIP(k)
			if ip == nil {
				return errors.New("mapping's first level key should be ip .")
			}
			err := checkMapping(v)
			if err != nil {
				return err
			}
		}
	} else {
		parts := strings.Split(nodePath, "/")
		ip := net.ParseIP(parts[1])
		if ip == nil {
			return errors.New("mapping's first level key should be ip .")
		}
		// nodePath: /ip
		if len(parts) == 2 {
			err := checkMapping(data)
			if err != nil {
				return err
			}
		} else {
			// nodePath: /ip/{key:.*}
			_, isMap := data.(map[string]interface{})
			if isMap {
				err := checkMapping(data)
				if err != nil {
					return err
				}
			} else {
				err := checkMappingPath(data)
				if err != nil {
					return err
				}
			}
		}
	}
	return r.storeClient.PutMapping(nodePath, data, replace)
}

func (r *MetadataRepo) DeleteMapping(nodePath string, subs ...string) error {
	err := checkSubs(subs)
	if err != nil {
		return err
	}
	if len(subs) > 0 {
		for _, sub := range subs {
			subPath := path.Join(nodePath, sub)
			_, v := r.mapping.Get(subPath)
			// if subPath mapping not exist, just ignore.
			if v != nil {
				_, dir := v.(map[string]interface{})
				err = r.storeClient.DeleteMapping(subPath, dir)
				if err != nil {
					return err
				}
			}
		}
		return nil
	} else {
		_, v := r.mapping.Get(nodePath)
		if v != nil {
			_, dir := v.(map[string]interface{})
			return r.storeClient.DeleteMapping(nodePath, dir)
		}
		return nil
	}
}

func (r *MetadataRepo) DataVersion() int64 {
	return r.data.Version()
}

func checkSubs(subs []string) error {
	for _, sub := range subs {
		if strings.Index(sub, "/") >= 0 {
			return errors.New("Sub node must not a path.")
		}
	}
	return nil
}

func checkMapping(data interface{}) error {
	m, ok := data.(map[string]interface{})
	if !ok {
		return errors.New("mapping data should be json object.")
	}
	for k, v := range m {
		if strings.Index(k, "/") >= 0 {
			return errors.New("mapping key should not be path.")
		}
		_, isMap := v.(map[string]interface{})
		if isMap {
			err := checkMapping(v)
			if err != nil {
				return err
			}
		} else {
			err := checkMappingPath(v)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func checkMappingPath(v interface{}) error {
	vs, vok := v.(string)
	if !vok {
		return errors.New("mapping's value should be path .")
	}
	if vs == "" || vs[0] != '/' {
		return errors.New("mapping's value should be path .")
	}
	return nil
}
