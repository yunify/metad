package metadata

import (
	"github.com/yunify/metadata-proxy/backends"
	"github.com/yunify/metadata-proxy/log"
	"github.com/yunify/metadata-proxy/store"
	"path"
	"strings"
)

type Mapping map[string]string

type MetadataRepo struct {
	onlySelf        bool
	selfMapping     store.Store
	storeClient     backends.StoreClient
	metastore       store.Store
	metaStopChan    chan bool
	mappingStopChan chan bool
}

func New(onlySelf bool, storeClient backends.StoreClient) *MetadataRepo {
	metadataRepo := MetadataRepo{
		onlySelf:        onlySelf,
		selfMapping:     store.New(),
		storeClient:     storeClient,
		metastore:       store.New(),
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
	r.storeClient.Sync(r.metastore, r.metaStopChan)
}

func (r *MetadataRepo) startMappingSync() {
	r.storeClient.SyncSelfMapping(r.selfMapping, r.mappingStopChan)
}

func (r *MetadataRepo) ReSync() {
	log.Info("ReSync")
	//TODO lock
	r.StopSync()
	r.metastore.Delete("/")
	r.selfMapping.Delete("/")
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
		val, ok := r.metastore.Get(metapath)
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
			val, getOK := r.metastore.Get(subpath)
			if getOK {
				meta[k] = val
			} else {
				log.Warning("Can not get values from backend by path: %s", subpath)
			}
		}
		return meta, true
	} else {
		for k, v := range mapping {
			keyPath := path.Clean(path.Join("/", k))
			if strings.HasPrefix(metapath, keyPath) {
				metapath = path.Clean(path.Join("/", strings.TrimPrefix(metapath, keyPath)))
				nodePath := path.Clean(path.Join("/", v, metapath))
				return r.metastore.Get(nodePath)
			}
		}
		log.Warning("Can not get self metadata by clientIP: %s path: %s", clientIP, metapath)
		return nil, false
	}
}

func (r *MetadataRepo) SelfMapping(clientIP string) (map[string]string, bool) {
	mappingVal, ok := r.selfMapping.Get(clientIP)
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

func (r *MetadataRepo) Register(clientIP string, mapping Mapping) {
	log.Info("Register clientIP: %s, mapping: %v", clientIP, mapping)
	r.storeClient.RegisterSelfMapping(clientIP, mapping, true)
}

func (r *MetadataRepo) Unregister(clientIP string) {
	r.storeClient.UnregisterSelfMapping(clientIP)
	r.selfMapping.Delete(clientIP)
}
