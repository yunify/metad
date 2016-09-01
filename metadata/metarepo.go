package metadata

import (
	"errors"
	"fmt"
	"github.com/yunify/metad/backends"
	"github.com/yunify/metad/log"
	"github.com/yunify/metad/store"
	"net"
	"path"
	"reflect"
	"strings"
)

type MetadataRepo struct {
	onlySelf        bool
	mapping         store.Store
	storeClient     backends.StoreClient
	data            store.Store
	metaStopChan    chan bool
	mappingStopChan chan bool
}

func New(onlySelf bool, storeClient backends.StoreClient) *MetadataRepo {
	metadataRepo := MetadataRepo{
		onlySelf:        onlySelf,
		mapping:         store.New(),
		storeClient:     storeClient,
		data:            store.New(),
		metaStopChan:    make(chan bool),
		mappingStopChan: make(chan bool),
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

func (r *MetadataRepo) Root(clientIP string, metapath string) (interface{}, bool) {
	log.Debug("Get clientIP:%s metapath:%s", clientIP, metapath)

	metapath = path.Clean(path.Join("/", metapath))
	if r.onlySelf {
		if metapath == "/" {
			val := make(map[string]interface{})
			selfVal, ok := r.Self(clientIP, "/")
			if ok {
				val["self"] = selfVal
			}
			return val, true
		} else {
			return nil, false
		}
	} else {
		val := r.data.Get(metapath)
		if val == nil {
			return nil, false
		} else {
			if metapath == "/" {
				selfVal, ok := r.Self(clientIP, "/")
				if ok {
					mapVal, ok := val.(map[string]interface{})
					if ok {
						mapVal["self"] = selfVal
					}
				}
			}
			return val, true
		}
	}
}

func (r *MetadataRepo) Self(clientIP string, nodePath string) (interface{}, bool) {
	nodePath = path.Join("/", nodePath)
	log.Debug("Self nodePath:%s, clientIP:%s", nodePath, clientIP)
	mappingData, ok := r.GetMapping(path.Join("/", clientIP))
	if !ok {
		log.Warning("Can not find mapping for %s", clientIP)
		return nil, false
	}
	mapping, mok := mappingData.(map[string]interface{})
	if !mok {
		log.Warning("Mapping for %s is not a map, result:%v", clientIP, mappingData)
		return nil, false
	}
	return r.getMappingDatas(nodePath, mapping)
}

func (r *MetadataRepo) getMappingData(nodePath, link string) (interface{}, bool) {
	nodePath = path.Join(link, nodePath)
	data := r.data.Get(nodePath)
	log.Debug("getMappingData %s %v", nodePath, data != nil)
	return data
}

func (r *MetadataRepo) getMappingDatas(nodePath string, mapping map[string]interface{}) (interface{}, bool) {
	nodePath = path.Join("/", nodePath)
	paths := strings.Split(nodePath, "/")[1:] // trim first blank item
	// nodePath is "/"
	if paths[0] == "" {
		meta := make(map[string]interface{})
		for k, v := range mapping {
			submapping, isMap := v.(map[string]interface{})
			if isMap {
				val, getOK := r.getMappingDatas("/", submapping)
				if getOK {
					meta[k] = val
				} else {
					log.Warning("Can not get values from backend by mapping: %v", submapping)
				}
			} else {
				subNodePath := fmt.Sprintf("%v", v)
				val, getOK := r.getMappingData("/", subNodePath)
				if getOK {
					meta[k] = val
				} else {
					log.Warning("Can not get values from backend by mapping: %v", subNodePath)
				}
			}

		}
		return meta, true
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
			log.Debug("Can not find mapping for : %v, mapping:%v", nodePath, mapping)
			return nil, false
		}
	}
}

func (r *MetadataRepo) GetData(nodePath string) interface{} {
	return r.data.Get(nodePath)
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
			v := r.data.Get(subPath)
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
		v := r.data.Get(nodePath)
		if v != nil {
			_, dir := v.(map[string]interface{})
			return r.storeClient.Delete(nodePath, dir)
		}
		return nil
	}

}

func (r *MetadataRepo) GetMapping(nodePath string) interface{} {
	return r.mapping.Get(nodePath)
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
			v := r.mapping.Get(subPath)
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
		v := r.mapping.Get(nodePath)
		if v != nil {
			_, dir := v.(map[string]interface{})
			return r.storeClient.DeleteMapping(nodePath, dir)
		}
		return nil
	}
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
