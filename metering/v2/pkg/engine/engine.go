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

package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/InVisionApp/go-health/v2"
	"github.com/go-logr/logr"
	"github.com/google/wire"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/filter"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/stores"
	pkgtypes "github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/types"
	marketplacev1beta1client "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/generated/clientset/versioned/typed/marketplace/v1beta1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Engine struct {
	log        logr.Logger
	runnables  Runnables
	health     *health.Health
	cancelFunc context.CancelFunc
}

func ProvideEngine(
	log logr.Logger,
	runnables Runnables,
) *Engine {
	h := health.New()
	return &Engine{
		log:       log,
		runnables: runnables,
		health:    h,
	}
}

func (e *Engine) Start(ctx context.Context) error {
	e.Stop() // just incase this is the second time

	localCtx, cancel := context.WithCancel(ctx)
	e.cancelFunc = cancel

	for i := range e.runnables {
		runnable := e.runnables[i]
		e.log.Info("starting runnable", "runnable", fmt.Sprintf("%T", runnable))
		err := runnable.Start(localCtx)

		if err != nil {
			return err
		}
	}

	return e.health.Start()
}

func (e *Engine) Stop() {
	if e.cancelFunc == nil {
		return
	}

	e.cancelFunc()
}

type reflectorConfig struct {
	expectedType client.Object
	lister       func(string) cache.ListerWatcher
}

type ListerRunnable struct {
	reflectorConfig
	namespace  string
	cancelFunc context.CancelFunc

	Store cache.Store
}

func (p *ListerRunnable) Start(ctx context.Context) error {
	if p.Store == nil {
		return errors.New("cache is nil")
	}

	var localCtx context.Context
	localCtx, p.cancelFunc = context.WithCancel(ctx)
	lister := p.lister(p.namespace)
	reflector := cache.NewReflector(lister, p.expectedType, p.Store, 0)
	go reflector.Run(localCtx.Done())
	return nil
}

func (p *ListerRunnable) Stop() {
	p.cancelFunc()
}

type StoreRunnable struct {
	Store      cache.Store
	Reflectors Runnables
	ResyncTime time.Duration

	startContext context.Context
	log          logr.Logger

	cancelFunc context.CancelFunc
}

func (p *StoreRunnable) Start(ctx context.Context) error {
	if p.startContext == nil {
		p.startContext = ctx
	}

	var localCtx context.Context
	localCtx, p.cancelFunc = context.WithCancel(p.startContext)

	for i := range p.Reflectors {
		p.log.Info(fmt.Sprintf("starting reflector %T", p.Reflectors[i]))
		err := p.Reflectors[i].Start(localCtx)

		if err != nil {
			return err
		}
	}

	if p.ResyncTime != 0 {
		ticker := time.NewTicker(p.ResyncTime)

		go func() {
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					err := p.Store.Resync()
					if err != nil {
						p.log.Error(err, "error resyncing store")
					}
				case <-localCtx.Done():
					return
				}
			}
		}()
	}

	return nil
}

type MeterDefinitionDictionaryStoreRunnable struct {
	StoreRunnable
}

func ProvideMeterDefinitionDictionaryStoreRunnable(
	kubeClient clientset.Interface,
	nses pkgtypes.Namespaces,
	c *marketplacev1beta1client.MarketplaceV1beta1Client,
	store *stores.MeterDefinitionDictionary,
	log logr.Logger,
) *MeterDefinitionDictionaryStoreRunnable {
	runnables := []Runnable{}

	for _, ns := range nses {
		runnables = append(runnables, provideMeterDefinitionListerRunnable(ns, c, store))
	}

	return &MeterDefinitionDictionaryStoreRunnable{
		StoreRunnable: StoreRunnable{
			Store:      store,
			Reflectors: runnables,
			log:        log.WithName("dictionary"),
		},
	}
}

type MeterDefinitionStoreRunnable struct {
	StoreRunnable
}

func ProvideMeterDefinitionStoreRunnable(
	nses pkgtypes.Namespaces,
	log logr.Logger,
	ns *filter.NamespaceWatcher,
	listWatchers MeterDefinitionStoreListWatchers,
	store *stores.MeterDefinitionStore,
) *MeterDefinitionStoreRunnable {
	return &MeterDefinitionStoreRunnable{
		StoreRunnable: StoreRunnable{
			Store: store,
			log:   log.WithName("mdefstore"),
			Reflectors: []Runnable{
				&NamespacedCachedListers{
					watcher:     ns,
					log:         log.WithName("namespaced-cached-lister"),
					listers:     map[string][]RunAndStop{},
					makeListers: ListWatchers(listWatchers),
				},
			},
		},
	}
}

var EngineSet = wire.NewSet(
	ProvideEngine,
	stores.NewMeterDefinitionDictionary,
	stores.NewMeterDefinitionStore,
	ProvideMeterDefinitionStoreListWatchers,
	wire.Struct(new(filter.MeterDefinitionLookupFilterFactory), "*"),
)
