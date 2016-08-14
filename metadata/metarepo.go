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
	onlySelf    bool
	selfMapping map[string]Mapping
	storeClient backends.StoreClient
	Metastore   store.Store
	stopChan    chan bool
}

func New(onlySelf bool, selfMapping map[string]Mapping, storeClient backends.StoreClient, metastore store.Store) *MetadataRepo {
	metadataRepo := MetadataRepo{
		onlySelf:    onlySelf,
		selfMapping: selfMapping,
		storeClient: storeClient,
		Metastore:   metastore,
		stopChan:    make(chan bool),
	}
	return &metadataRepo
}

func (r *MetadataRepo) SetOnlySelf(onlySelf bool) {
	r.onlySelf = onlySelf
}

func (r *MetadataRepo) StartSync() {
	r.storeClient.Sync(r.Metastore, r.stopChan)
}

func (r *MetadataRepo) ReSync() {
	r.stopChan <- true
	r.Metastore.Delete("/")
	r.storeClient.Sync(r.Metastore, r.stopChan)
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
		val, ok := r.Metastore.Get(metapath)
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
	metakeys, ok := r.selfMapping[clientIP]
	if !ok {
		return nil, false
	}
	if metapath == "/" {
		meta := make(map[string]interface{})
		for k, v := range metakeys {
			subpath := path.Clean(path.Join("/", v))
			val, getOK := r.Metastore.Get(subpath)
			if getOK {
				meta[k] = val
			} else {
				log.Warning("Can not get values from backend by path: %s", subpath)
			}
		}
		return meta, true
	} else {
		for k, v := range metakeys {
			keyPath := path.Clean(path.Join("/", k))
			if strings.HasPrefix(metapath, keyPath) {
				metapath = path.Clean(path.Join("/", strings.TrimPrefix(metapath, keyPath)))
				nodePath := path.Clean(path.Join("/", v, metapath))
				return r.Metastore.Get(nodePath)
			}
		}
		log.Warning("Can not get self metadata by clientIP: %s path: %s", clientIP, metapath)
		return nil, false
	}
}

func (r *MetadataRepo) SelfMapping(clientIP string) (map[string]string, bool) {
	val, ok := r.selfMapping[clientIP]
	return val, ok
}

func (r *MetadataRepo) Register(clientIP string, mapping Mapping) {
	r.selfMapping[clientIP] = mapping
}

func (r *MetadataRepo) Unregister(clientIP string) {
	delete(r.selfMapping, clientIP)
}
