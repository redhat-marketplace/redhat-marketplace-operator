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
	"sync"
	"time"

	"github.com/InVisionApp/go-health/v2"
	"github.com/go-logr/logr"
	"github.com/google/wire"
	monitoringv1client "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/internal/metrics"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/filter"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/stores"
	pkgtypes "github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/types"
	marketplacev1beta1client "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/generated/clientset/versioned/typed/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/managers"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Engine struct {
	store            *stores.MeterDefinitionStore
	namespaces       pkgtypes.Namespaces
	kubeClient       clientset.Interface
	monitoringClient *monitoringv1client.MonitoringV1Client
	dictionary       *stores.MeterDefinitionDictionary
	mktplaceClient   *marketplacev1beta1client.MarketplaceV1beta1Client
	promtheusData    *metrics.PrometheusData
	runnables        Runnables
	log              logr.Logger
	health           *health.Health

	mainContext  *context.Context
	localContext *context.Context
	cancelFunc   context.CancelFunc
}

func ProvideEngine(
	store *stores.MeterDefinitionStore,
	namespaces pkgtypes.Namespaces,
	log logr.Logger,
	kubeClient clientset.Interface,
	monitoringClient *monitoringv1client.MonitoringV1Client,
	dictionary *stores.MeterDefinitionDictionary,
	mktplaceClient *marketplacev1beta1client.MarketplaceV1beta1Client,
	runnables Runnables,
	promtheusData *metrics.PrometheusData,
	cache managers.CacheIsStarted,
) *Engine {
	h := health.New()
	return &Engine{
		store:            store,
		log:              log,
		namespaces:       namespaces,
		kubeClient:       kubeClient,
		monitoringClient: monitoringClient,
		dictionary:       dictionary,
		mktplaceClient:   mktplaceClient,
		runnables:        runnables,
		health:           h,
		promtheusData:    promtheusData,
	}
}

func (e *Engine) Start(ctx context.Context) error {
	if e.cancelFunc != nil {
		e.mainContext = nil
		e.localContext = nil
		e.cancelFunc()
	}

	localCtx, cancel := context.WithCancel(ctx)
	e.mainContext = &ctx
	e.localContext = &localCtx
	e.cancelFunc = cancel

	for i := range e.runnables {
		runnable := e.runnables[i]
		e.log.Info("starting runnable", "runnable", fmt.Sprintf("%T", runnable))
		wg := sync.WaitGroup{}

		wg.Add(1)

		go func() {
			wg.Done()
			runnable.Start(localCtx)
		}()

		wg.Wait()
	}

	return e.health.Start()
}

type reflectorConfig struct {
	expectedType client.Object
	lister       func(string) cache.ListerWatcher

	startContext context.Context
	cancelFunc   context.CancelFunc
}

type ListerRunnable struct {
	reflectorConfig
	namespace string

	Store cache.Store
}

func (p *ListerRunnable) Start(ctx context.Context) error {
	if p.Store == nil {
		return errors.New("cache is nil")
	}

	if p.startContext == nil {
		p.startContext = ctx
	}

	var localCtx context.Context
	localCtx, p.cancelFunc = context.WithCancel(p.startContext)
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
					p.Store.Resync()
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
)
