package metadata

import (
	log "github.com/Sirupsen/logrus"
	"github.com/yunify/metadata-proxy/backends"
	"github.com/yunify/metadata-proxy/store"
	"path"
	"strings"
)

type MetadataRepo struct {
	prefix      string
	selfMapping map[string]map[string]string
	storeClient backends.StoreClient
	Metastore   store.Store
	stopChan    chan bool
}

func New(prefix string, selfMapping map[string]map[string]string, storeClient backends.StoreClient, metastore store.Store) *MetadataRepo {
	metadataRepo := MetadataRepo{
		prefix:      prefix,
		selfMapping: selfMapping,
		storeClient: storeClient,
		Metastore:   metastore,
		stopChan:    make(chan bool),
	}
	return &metadataRepo
}

func (r *MetadataRepo) StartSync() {
	r.storeClient.Sync(r.Metastore, r.stopChan)
}

func (r *MetadataRepo) ReSync() {
	r.stopChan <- true
	r.Metastore.Delete("/")
	r.storeClient.Sync(r.Metastore, r.stopChan)
}

func (r *MetadataRepo) Get(metapath string) (interface{}, bool) {
	metapath = path.Clean(path.Join("/", metapath))
	return r.Metastore.Get(metapath)
}

func (r *MetadataRepo) GetSelf(clientIP string, metapath string) (interface{}, bool) {
	metapath = path.Clean(path.Join("/", metapath))
	metakeys, ok := r.selfMapping[clientIP]
	if !ok {
		return nil, false
	}
	if metapath == "/" {
		meta := make(map[string]interface{})
		for k, v := range metakeys {
			nodePath := path.Clean(path.Join("/", v))
			val, getOK := r.Metastore.Get(nodePath)
			if getOK {
				meta[k] = val
			} else {
				log.Warnf("Can not get values from backend by path: %s", nodePath)
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
		log.Warnf("Can not get self metadata by clientIP: %s path: %s", clientIP, metapath)
		return nil, false
	}
}

func (r *MetadataRepo) SelfMapping(clientIP string) (map[string]string, bool) {
	val, ok := r.selfMapping[clientIP]
	return val, ok
}

func (r *MetadataRepo) Register(clientIP string, mapping map[string]string) {
	r.selfMapping[clientIP] = mapping
}

func (r *MetadataRepo) Unregister(clientIP string) {
	delete(r.selfMapping, clientIP)
}
