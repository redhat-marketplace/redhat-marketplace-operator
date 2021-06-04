package dictionary

import (
	"fmt"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/allegro/bigcache"
	"github.com/gotidy/ptr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/filter"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var initLookupCache sync.Once
var lookupCache *resultCache

func init() {
	initLookupCache.Do(func() {
		lookupCache = utils.Must(func() (interface{}, error) {
			cache, err := bigcache.NewBigCache(bigcache.DefaultConfig(10 * time.Minute))

			if err != nil {
				return nil, err
			}

			return &resultCache{
				cache: cache,
				mutex: sync.RWMutex{},
			}, nil
		}).(*resultCache)
	})
}

type resultCache struct {
	cache *bigcache.BigCache
	mutex sync.RWMutex
}

func (r *resultCache) cacheKey(filter *filter.MeterDefinitionLookupFilter, obj metav1.Object) string {
	return fmt.Sprintf("mdefuid:%s::uid:%s", filter.MeterDefUID, string(obj.GetUID()))
}

func (r *resultCache) Get(filter *filter.MeterDefinitionLookupFilter, obj interface{}) *bool {
	o, ok := obj.(metav1.Object)

	if !ok {
		return nil
	}

	r.mutex.RLock()
	key := r.cacheKey(filter, o)
	entry, err := r.cache.Get(key)
	r.mutex.RUnlock()

	if errors.Is(err, bigcache.ErrEntryNotFound) {
		return nil
	}

	return ptr.Bool(entry[0] == 1)
}

func (r *resultCache) Set(filter *filter.MeterDefinitionLookupFilter, obj interface{}, result bool) error {
	o, ok := obj.(metav1.Object)

	if !ok {
		return errors.New("type is not a metav1 Object")
	}

	r.mutex.Lock()
	key := r.cacheKey(filter, o)
	val := []byte{0}

	if result {
		val[0] = 1
	} else {
		val[0] = 0
	}

	err := r.cache.Set(key, val)
	r.mutex.Unlock()
	return err
}

func (r *resultCache) Delete(filter *filter.MeterDefinitionLookupFilter, obj interface{}) error {
	o, ok := obj.(metav1.Object)

	if !ok {
		return errors.New("type is not a metav1 Object")
	}

	r.mutex.Lock()
	key := r.cacheKey(filter, o)

	err := r.cache.Delete(key)

	r.mutex.Unlock()
	return err
}
