package metadata

import (
	"errors"
	"github.com/yunify/metadata-proxy/backends"
	"github.com/yunify/metadata-proxy/log"
	"github.com/yunify/metadata-proxy/store"
	"net"
	"path"
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

func (r *MetadataRepo) Register(clientIP string, mapping map[string]string) {
	log.Info("Register clientIP: %s, mapping: %v", clientIP, mapping)
	r.storeClient.UpdateMapping(clientIP, mapping, true)
}

func (r *MetadataRepo) Unregister(clientIP string) {
	r.storeClient.DeleteMapping(clientIP)
	r.mapping.Delete(clientIP)
}

func (r *MetadataRepo) UpdateData(nodePath string, data interface{}, replace bool) error {
	return r.storeClient.Set(nodePath, data, replace)
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
		err := checkPath(v)
		if err != nil {
			return err
		}
	}
	return nil
}

func checkPath(v interface{}) error {
	vs, vok := v.(string)
	if !vok {
		return errors.New("mapping's value should be path .")
	}
	if vs == "" || vs[0] != '/' {
		return errors.New("mapping's value should be path .")
	}
	return nil
}

func (r *MetadataRepo) UpdateMapping(nodePath string, data interface{}, replace bool) error {
	nodePath = path.Clean(path.Join("/", nodePath))
	if nodePath == "/" {
		m, ok := data.(map[string]interface{})
		if !ok {
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
		// mapping only allow 2 level ip/key
		if len(parts) > 2 {
			return errors.New("mapping path only support two level.")
		}
		ip := net.ParseIP(parts[0])
		if ip == nil {
			return errors.New("mapping's first level key should be ip .")
		}
		if len(parts) == 1 {
			err := checkMapping(data)
			if err != nil {
				return err
			}
		} else {
			err := checkPath(data)
			if err != nil {
				return err
			}
		}
	}
	return r.storeClient.UpdateMapping(nodePath, data, replace)
}

func (r *MetadataRepo) DeleteMapping(nodePath string) error {
	return r.storeClient.DeleteMapping(nodePath)
}

func (r *MetadataRepo) GetMapping(nodePath string) (interface{}, bool) {
	return r.mapping.Get(nodePath)
}
