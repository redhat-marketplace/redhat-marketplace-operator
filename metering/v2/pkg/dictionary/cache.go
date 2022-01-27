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
	"os"
	"strconv"
	"sync"
	"time"

	"emperror.dev/errors"
	bigcache "github.com/allegro/bigcache/v3"
	xxhash "github.com/cespare/xxhash/v2"
	"github.com/gotidy/ptr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/filter"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/sasha-s/go-deadlock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var initLookupCache sync.Once
var lookupCache *resultCache

var result_t = []byte{1}
var result_f = []byte{0}

const cacheTimeout = 30 * time.Minute
const pct = 0.35

func init() {
	initLookupCache.Do(func() {
		lookupCache = utils.Must(func() (interface{}, error) {
			limit, _ := strconv.Atoi(os.Getenv("MEMORY_LIMIT"))
			limit = int(float64(limit) * pct)
			cache, err := bigcache.NewBigCache(bigcache.Config{
				Shards:             1024,
				LifeWindow:         cacheTimeout,
				CleanWindow:        1 * time.Second,
				MaxEntriesInWindow: 1000 * 10 * 60,
				MaxEntrySize:       500,
				StatsEnabled:       false,
				Verbose:            true,
				HardMaxCacheSize:   limit,
				Hasher:             newHasher(),
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

	var err error
	if result {
		err = r.cache.Set(key, result_t)
	} else {
		err = r.cache.Set(key, result_f)
	}

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

type hasher struct {
	digest *xxhash.Digest
}

func newHasher() *hasher {
	var h hasher
	h.digest = xxhash.New()
	return &h
}

func (h *hasher) Sum64(s string) uint64 {
	h.digest.Write([]byte(s))
	sum := h.digest.Sum64()
	h.digest.Reset()
	return sum
}
