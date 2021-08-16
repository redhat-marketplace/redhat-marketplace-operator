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
	pkgtypes "github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/types"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/client"
	"k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const IndexRazee = "razee"

type RazeeStores struct {
	pkgtypes.Stores
}

type RazeeStore struct {
	*cache.DeltaFIFO
	store   cache.Store
	keyFunc cache.KeyFunc

	ctx    context.Context
	log    logr.Logger
	scheme *runtime.Scheme

	sync.RWMutex

	// kubeClient to query kube
	kubeClient clientset.Interface

	findOwner *rhmclient.FindOwnerHelper
}

var _ cache.Store = &RazeeStore{}

type RazeeStoreGroup struct {
	Store  *RazeeStore
	Stores RazeeStores
}

func NewRazeeStore(
	ctx context.Context,
	log logr.Logger,
	kubeClient clientset.Interface,
	findOwner *rhmclient.FindOwnerHelper,
	scheme *runtime.Scheme,
) RazeeStoreGroup {
	keyFunc := pkgtypes.GVKNamespaceKeyFunc(scheme)
	store := cache.NewStore(keyFunc)
	delta := cache.NewDeltaFIFOWithOptions(cache.DeltaFIFOOptions{
		KeyFunction:           keyFunc,
		KnownObjects:          store,
		EmitDeltaTypeReplaced: true,
	})

	primary := pkgtypes.PrimaryStore{
		Store: store,
	}

	fifo := &RazeeStore{
		ctx:        ctx,
		log:        log.WithName("razee_store").V(4),
		scheme:     scheme,
		kubeClient: kubeClient,
		findOwner:  findOwner,
		DeltaFIFO:  delta,
		keyFunc:    keyFunc,
	}

	stores := RazeeStores{
		Stores: pkgtypes.Stores{
			fifo, primary, // want fifo first so we get all events
		},
	}

	return RazeeStoreGroup{
		Store:  fifo,
		Stores: stores,
	}
}
