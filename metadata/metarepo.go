package metadata

import (
	"errors"
	"github.com/yunify/metadata-proxy/backends"
	"github.com/yunify/metadata-proxy/log"
	"github.com/yunify/metadata-proxy/store"
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

func (r *MetadataRepo) Get(clientIP string, metapath string) (interface{}, bool) {
	log.Debug("Get clientIP:%s metapath:%s", clientIP, metapath)

	metapath = path.Clean(path.Join("/", metapath))
	if r.onlySelf {
		if metapath == "/" {
			val := make(map[string]interface{})
			selfVal, ok := r.GetSelf(clientIP, "/")
			if ok {
				val["self"] = selfVal
			}
			return val, true
		} else {
			return nil, false
		}
	} else {
		val, ok := r.data.Get(metapath)
		if !ok {
			return nil, false
		} else {
			if metapath == "/" {
				selfVal, ok := r.GetSelf(clientIP, "/")
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

func (r *MetadataRepo) GetSelf(clientIP string, metapath string) (interface{}, bool) {
	metapath = path.Clean(path.Join("/", metapath))
	log.Debug("GetSelf clientIP:%s metapath:%s", clientIP, metapath)
	mapping, ok := r.SelfMapping(clientIP)
	if !ok {
		log.Warning("Can not find mapping for %s", clientIP)
		return nil, false
	}
	if metapath == "/" {
		meta := make(map[string]interface{})
		for k, v := range mapping {
			subpath := path.Clean(path.Join("/", v))
			val, getOK := r.data.Get(subpath)
			if getOK {
				meta[k] = val
			} else {
				log.Warning("Can not get values from backend by path: %s", subpath)
			}
		}
		return meta, true
	} else {
		//for avoid to miss match /nodes and /node, so add "/" to end.
		metapath = metapath + "/"
		for k, v := range mapping {
			keyPath := path.Clean(path.Join("/", k)) + "/"
			if strings.HasPrefix(metapath, keyPath) {
				metapath = path.Clean(path.Join("/", strings.TrimPrefix(metapath, keyPath)))
				nodePath := path.Clean(path.Join("/", v, metapath))
				result, rok := r.data.Get(nodePath)
				log.Debug("Self key:%s, nodePath:%s, ok:%v, result:%v", keyPath, nodePath, rok, result)
				return result, rok
			}
		}
		log.Warning("Can not get self metadata by clientIP: %s path: %s", clientIP, metapath)
		return nil, false
	}
}

func (r *MetadataRepo) SelfMapping(clientIP string) (map[string]string, bool) {
	mappingVal, ok := r.mapping.Get(clientIP)
	if !ok {
		return nil, false
	}
	mapping := make(map[string]string)
	for k, v := range mappingVal.(map[string]interface{}) {
		path, ok := v.(string)
		if !ok {
			log.Warning("self mapping value should be string : %v", v)
			continue
		}
		mapping[k] = path
	}
	return mapping, true
}

func (r *MetadataRepo) GetData(nodePath string) (interface{}, bool) {
	return r.data.Get(nodePath)
}

func (r *MetadataRepo) UpdateData(nodePath string, data interface{}, replace bool) error {
	return r.storeClient.Set(nodePath, data, replace)
}

func (r *MetadataRepo) DeleteData(nodePath string, subs ...string) error {
	err := checkSubs(subs)
	if err != nil {
		return err
	}
	if len(subs) > 0 {
		for _, sub := range subs {
			subPath := path.Join(nodePath, sub)
			v, ok := r.data.Get(subPath)
			// if subPath metadata not exist, just ignore.
			if ok {
				_, dir := v.(map[string]interface{})
				err = r.storeClient.Delete(subPath, dir)
				if err != nil {
					return err
				}
			}
		}
		return nil
	} else {
		v, ok := r.data.Get(nodePath)
		if ok {
			_, dir := v.(map[string]interface{})
			return r.storeClient.Delete(nodePath, dir)
		}
		return nil
	}

}

func (r *MetadataRepo) GetMapping(nodePath string) (interface{}, bool) {
	return r.mapping.Get(nodePath)
}

func (r *MetadataRepo) UpdateMapping(nodePath string, data interface{}, replace bool) error {
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
		// mapping only allow 2 level /ip/key split result  [,ip,key] len == 3
		if len(parts) > 3 {
			return errors.New("mapping path only support two level.")
		}
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
			// nodePath: /ip/key
			err := checkMappingPath(data)
			if err != nil {
				return err
			}
		}
	}
	return r.storeClient.UpdateMapping(nodePath, data, replace)
}

func (r *MetadataRepo) DeleteMapping(nodePath string, subs ...string) error {
	err := checkSubs(subs)
	if err != nil {
		return err
	}
	if len(subs) > 0 {
		for _, sub := range subs {
			subPath := path.Join(nodePath, sub)
			v, ok := r.mapping.Get(subPath)
			// if subPath mapping not exist, just ignore.
			if ok {
				_, dir := v.(map[string]interface{})
				err = r.storeClient.DeleteMapping(subPath, dir)
				if err != nil {
					return err
				}
			}
		}
		return nil
	} else {
		v, ok := r.mapping.Get(nodePath)
		if ok {
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
		err := checkMappingPath(v)
		if err != nil {
			return err
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
