// Copyright 2021 IBM Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dictionary

import (
	"fmt"
	"sync"
	"time"

	"emperror.dev/errors"
	bigcache "github.com/allegro/bigcache/v3"
	"github.com/gotidy/ptr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/filter"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/sasha-s/go-deadlock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var initLookupCache sync.Once
var lookupCache *resultCache

const cacheTimeout = 30 * time.Minute

func init() {
	initLookupCache.Do(func() {
		lookupCache = utils.Must(func() (interface{}, error) {
			cache, err := bigcache.NewBigCache(bigcache.Config{
				Shards:             1024,
				LifeWindow:         cacheTimeout,
				CleanWindow:        0,
				MaxEntriesInWindow: 1000 * 10 * 60,
				MaxEntrySize:       500,
				Verbose:            true,
				HardMaxCacheSize:   100,
			})

			if err != nil {
				return nil, err
			}

			return &resultCache{
				cache: cache,
				mutex: deadlock.RWMutex{},
			}, nil
		}).(*resultCache)
	})
}

type resultCache struct {
	cache *bigcache.BigCache
	mutex deadlock.RWMutex
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
