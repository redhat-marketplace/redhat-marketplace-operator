// Copyright 2020 IBM Corp.
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

package razee

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/client"
	"k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const IndexRazee = "razee"

type RazeeStores = map[string]*RazeeStore

type RazeeStore struct {
	indexStore cache.Indexer
	delta      *cache.DeltaFIFO
	keyFunc    cache.KeyFunc

	ctx    context.Context
	log    logr.Logger
	scheme *runtime.Scheme

	sync.RWMutex

	// kubeClient to query kube
	kubeClient clientset.Interface

	findOwner *rhmclient.FindOwnerHelper
}

func NewRazeeStore(
	ctx context.Context,
	log logr.Logger,
	kubeClient clientset.Interface,
	findOwner *rhmclient.FindOwnerHelper,
	scheme *runtime.Scheme,
) *RazeeStore {
	keyFunc := cache.MetaNamespaceKeyFunc

	storeIndexers := cache.Indexers{
		IndexRazee: cache.MetaNamespaceIndexFunc,
	}

	store := cache.NewIndexer(keyFunc, storeIndexers)
	delta := cache.NewDeltaFIFO(keyFunc, store)
	return &RazeeStore{
		ctx:        ctx,
		log:        log.WithName("razee_store").V(4),
		scheme:     scheme,
		kubeClient: kubeClient,
		findOwner:  findOwner,
		delta:      delta,
		indexStore: store,
		keyFunc:    keyFunc,
	}
}

func (s *RazeeStore) DeltaStore() *cache.DeltaFIFO {
	return s.delta
}
