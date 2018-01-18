// Copyright 2018 Yunify Inc. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

package metadata

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/yunify/metad/backends"
	"github.com/yunify/metad/log"
	"github.com/yunify/metad/store"
	"github.com/yunify/metad/util"
	"github.com/yunify/metad/util/flatmap"
)

const DEFAULT_WATCH_BUF_LEN = 100

type MetadataRepo struct {
	mapping            store.Store
	storeClient        backends.StoreClient
	data               store.Store
	accessStore        store.AccessStore
	metaStopChan       chan bool
	mappingStopChan    chan bool
	accessRuleStopChan chan bool
	timerPool          *util.TimerPool
}

func New(storeClient backends.StoreClient) *MetadataRepo {
	metadataRepo := MetadataRepo{
		mapping:            store.New(),
		storeClient:        storeClient,
		data:               store.New(),
		accessStore:        store.NewAccessStore(),
		metaStopChan:       make(chan bool),
		mappingStopChan:    make(chan bool),
		accessRuleStopChan: make(chan bool),
		timerPool:          util.NewTimerPool(100 * time.Millisecond),
	}
	return &metadataRepo
}

func (r *MetadataRepo) StartSync() {
	log.Info("Start Sync")
	r.startMetaSync()
	r.startMappingSync()
	r.startAccessRuleSync()
}

func (r *MetadataRepo) startMetaSync() {
	r.storeClient.Sync(r.data, r.metaStopChan)
}

func (r *MetadataRepo) startMappingSync() {
	r.storeClient.SyncMapping(r.mapping, r.mappingStopChan)
}

func (r *MetadataRepo) startAccessRuleSync() {
	r.storeClient.SyncAccessRule(r.accessStore, r.accessRuleStopChan)
}

func (r *MetadataRepo) StopSync() {
	log.Info("Stop Sync")
	r.metaStopChan <- true
	r.mappingStopChan <- true
	r.accessRuleStopChan <- true
	time.Sleep(1 * time.Second)
	r.data.Destroy()
	time.Sleep(1 * time.Second)
	r.mapping.Destroy()
}

func (r *MetadataRepo) getAccessTree(clientIP string) store.AccessTree {
	accessTree := r.accessStore.Get(clientIP)
	//for compatible with old version, auto convert mapping to AccessRule
	if accessTree == nil {
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
		flattenMapping := flatmap.Flatten(mapping)
		rules := []store.AccessRule{}
		for _, dataPath := range flattenMapping {
			rules = append(rules, store.AccessRule{Path: dataPath, Mode: store.AccessModeRead})
		}
		accessTree = store.NewAccessTree(rules)
	}
	return accessTree
}

func (r *MetadataRepo) Root(clientIP string, nodePath string) (currentVersion int64, val interface{}) {
	if clientIP == "" {
		panic(errors.New("clientIP must not be empty."))
	}
	nodePath = path.Join("/", nodePath)
	accessTree := r.getAccessTree(clientIP)
	if accessTree == nil {
		return
	}
	traveller := r.data.Traveller(accessTree)
	defer traveller.Close()
	if !traveller.Enter(nodePath) {
		return
	}
	currentVersion = traveller.GetVersion()
	val = traveller.GetValue()
	if val != nil && nodePath == "/" {
		selfVal := r.self(clientIP, "/", traveller)
		if selfVal != nil {
			mapVal, ok := val.(map[string]interface{})
			if ok {
				mapVal["self"] = selfVal
			}
		}
	}
	return
}

func (r *MetadataRepo) Watch(ctx context.Context, clientIP string, nodePath string) interface{} {
	nodePath = path.Join("/", nodePath)
	w := r.data.Watch(nodePath, DEFAULT_WATCH_BUF_LEN)
	return r.changeToResult(w, ctx.Done())
}

var TIMER_NIL *time.Timer = &time.Timer{C: nil}

func (r *MetadataRepo) changeToResult(watcher store.Watcher, stopChan <-chan struct{}) interface{} {
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
				if timer.C != nil {
					r.timerPool.ReleaseTimer(timer)
				}
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

func (r *MetadataRepo) WatchSelf(ctx context.Context, clientIP string, nodePath string) interface{} {
	nodePath = path.Join(clientIP, "/", nodePath)
	if log.IsDebugEnable() {
		log.Debug("WatchSelf nodePath: %s", nodePath)
	}
	mappingData := r.GetMapping(nodePath)
	if mappingData == nil {
		return nil
	}
	mappingWatcher := r.mapping.Watch(nodePath, DEFAULT_WATCH_BUF_LEN)
	defer mappingWatcher.Remove()

	stopChan := make(chan struct{})

	go func() {
		select {
		case _, ok := <-mappingWatcher.EventChan():
			if ok {
				close(stopChan)
			}
		case <-ctx.Done():
			close(stopChan)
		}
	}()

	mapping, mok := mappingData.(map[string]interface{})
	if !mok {
		dataNodePath := fmt.Sprintf("%s", mappingData)
		//log.Debug("watcher: %v", dataNodePath)
		w := r.data.Watch(dataNodePath, DEFAULT_WATCH_BUF_LEN)
		return r.changeToResult(w, stopChan)
	} else {
		flatMapping := flatmap.Flatten(mapping)
		watchers := make(map[string]store.Watcher)
		for k, v := range flatMapping {
			watchers[k] = r.data.Watch(v, DEFAULT_WATCH_BUF_LEN)
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

	accessTree := r.getAccessTree(clientIP)
	if accessTree == nil {
		return nil
	}
	traveller := r.data.Traveller(accessTree)
	defer traveller.Close()
	return r.self(clientIP, nodePath, traveller)
}

func (r *MetadataRepo) self(clientIP string, nodePath string, traveller store.Traveller) interface{} {
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
	return r.getMappingDatas(nodePath, mapping, traveller)
}

func (r *MetadataRepo) getMappingData(nodePath, link string, traveller store.Traveller) interface{} {
	nodePath = path.Join(link, nodePath)
	if traveller.Enter(nodePath) {
		val := traveller.GetValue()
		traveller.BackToRoot()
		return val
	}
	return nil
}

func (r *MetadataRepo) getMappingDatas(nodePath string, mapping map[string]interface{}, traveller store.Traveller) interface{} {
	nodePath = path.Join("/", nodePath)
	paths := strings.Split(nodePath, "/")[1:] // trim first blank item
	// nodePath is "/"
	if paths[0] == "" {
		meta := make(map[string]interface{})
		for k, v := range mapping {
			submapping, isMap := v.(map[string]interface{})
			if isMap {
				val := r.getMappingDatas("/", submapping, traveller)
				if val != nil {
					meta[k] = val
				} else {
					log.Warning("Can not get values from backend by mapping: %v", submapping)
				}
			} else {
				subNodePath := fmt.Sprintf("%v", v)
				val := r.getMappingData("/", subNodePath, traveller)
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
				return r.getMappingDatas(path.Join(paths[1:]...), submapping, traveller)
			} else {
				return r.getMappingData(path.Join(paths[1:]...), fmt.Sprintf("%v", elemValue), traveller)
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
			sub = strings.TrimSpace(sub)
			if sub == "" {
				continue
			}
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

func (r *MetadataRepo) PutAccessRule(rulesMap map[string][]store.AccessRule) error {
	for _, v := range rulesMap {
		err := store.CheckAccessRules(v)
		if err != nil {
			return err
		}
	}
	return r.storeClient.PutAccessRule(rulesMap)
}

func (r *MetadataRepo) DeleteAccessRule(hosts []string) error {
	if len(hosts) == 0 {
		return nil
	}
	return r.storeClient.DeleteAccessRule(hosts)
}

func (r *MetadataRepo) GetAccessRule(hosts []string) map[string][]store.AccessRule {
	return r.accessStore.GetAccessRule(hosts)
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
