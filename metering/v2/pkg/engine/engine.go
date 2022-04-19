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
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1client "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/internal/metrics"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/dictionary"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/meterdefinition"
	pkgtypes "github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/types"
	marketplacev1beta1client "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/generated/clientset/versioned/typed/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	corev1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Engine struct {
	store            *meterdefinition.MeterDefinitionStore
	namespaces       pkgtypes.Namespaces
	kubeClient       clientset.Interface
	monitoringClient *monitoringv1client.MonitoringV1Client
	dictionary       *dictionary.MeterDefinitionDictionary
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
	store *meterdefinition.MeterDefinitionStore,
	namespaces pkgtypes.Namespaces,
	log logr.Logger,
	kubeClient clientset.Interface,
	monitoringClient *monitoringv1client.MonitoringV1Client,
	dictionary *dictionary.MeterDefinitionDictionary,
	mktplaceClient *marketplacev1beta1client.MarketplaceV1beta1Client,
	runnables Runnables,
	promtheusData *metrics.PrometheusData,
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

	cancelFunc context.CancelFunc
}

type ListerRunnable struct {
	reflectorConfig
	namespaces pkgtypes.Namespaces

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

	for i := range p.namespaces {
		localNS := p.namespaces[i]
		lister := p.lister(localNS)
		reflector := cache.NewReflector(lister, p.expectedType, p.Store, 0)
		go reflector.Run(localCtx.Done())
	}

	return nil
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
		p.log.Info(fmt.Sprintf("starting reflector %T\n", p.Reflectors[i]))
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
	store *dictionary.MeterDefinitionDictionary,
	log logr.Logger,
) *MeterDefinitionDictionaryStoreRunnable {
	return &MeterDefinitionDictionaryStoreRunnable{
		StoreRunnable: StoreRunnable{
			Store:      store,
			ResyncTime: 5 * time.Minute,
			Reflectors: []Runnable{
				provideMeterDefinitionListerRunnable(nses, c, store),
			},
			log: log.WithName("dictionary"),
		},
	}
}

type MeterDefinitionSeenStoreRunnable struct {
	StoreRunnable
}

type MeterDefinitionStoreRunnable struct {
	StoreRunnable
}

func ProvideMeterDefinitionStoreRunnable(
	kubeClient clientset.Interface,
	nses pkgtypes.Namespaces,
	store *meterdefinition.MeterDefinitionStore,
	c *monitoringv1client.MonitoringV1Client,
	log logr.Logger,
) *MeterDefinitionStoreRunnable {
	return &MeterDefinitionStoreRunnable{
		StoreRunnable: StoreRunnable{
			Store:      store,
			ResyncTime: 5 * time.Minute,
			log:        log.WithName("mdefstore"),
			Reflectors: []Runnable{
				providePVCLister(kubeClient, nses, store),
				providePodListerRunnable(kubeClient, nses, store),
				provideServiceListerRunnable(kubeClient, nses, store),
				provideServiceMonitorListerRunnable(c, nses, store),
			},
		},
	}
}

type PVCListerRunnable struct {
	ListerRunnable
}

func providePVCLister(
	kubeClient clientset.Interface,
	nses pkgtypes.Namespaces,
	store cache.Store,
) *PVCListerRunnable {
	return &PVCListerRunnable{
		ListerRunnable: ListerRunnable{
			reflectorConfig: reflectorConfig{
				expectedType: &corev1.PersistentVolumeClaim{},
				lister:       CreatePVCListWatch(kubeClient),
			},
			Store:      store,
			namespaces: nses,
		},
	}
}

type PodListerRunnable struct {
	ListerRunnable
}

func providePodListerRunnable(
	kubeClient clientset.Interface,
	nses pkgtypes.Namespaces,
	store cache.Store,
) *PodListerRunnable {
	return &PodListerRunnable{
		ListerRunnable: ListerRunnable{
			reflectorConfig: reflectorConfig{
				expectedType: &corev1.Pod{},
				lister:       CreatePodListWatch(kubeClient),
			},
			namespaces: nses,
			Store:      store,
		},
	}
}

type ServiceListerRunnable struct {
	ListerRunnable
}

func provideServiceListerRunnable(
	kubeClient clientset.Interface,
	nses pkgtypes.Namespaces,
	store cache.Store,
) *ServiceListerRunnable {
	return &ServiceListerRunnable{
		ListerRunnable: ListerRunnable{
			reflectorConfig: reflectorConfig{
				expectedType: &corev1.Service{},
				lister:       CreateServiceListWatch(kubeClient),
			},
			namespaces: nses,
			Store:      store,
		},
	}
}

type ServiceMonitorListerRunnable struct {
	ListerRunnable
}

func provideServiceMonitorListerRunnable(
	c *monitoringv1client.MonitoringV1Client,
	nses pkgtypes.Namespaces,
	store cache.Store,
) *ServiceMonitorListerRunnable {
	return &ServiceMonitorListerRunnable{
		ListerRunnable: ListerRunnable{
			reflectorConfig: reflectorConfig{
				expectedType: &monitoringv1.ServiceMonitor{},
				lister:       CreateServiceMonitorListWatch(c),
			},
			namespaces: nses,
			Store:      store,
		},
	}
}

type MeterDefinitionListerRunnable struct {
	ListerRunnable
}

func provideMeterDefinitionListerRunnable(
	nses pkgtypes.Namespaces,
	c *marketplacev1beta1client.MarketplaceV1beta1Client,
	store cache.Store,
) *MeterDefinitionListerRunnable {
	return &MeterDefinitionListerRunnable{
		ListerRunnable: ListerRunnable{
			reflectorConfig: reflectorConfig{
				expectedType: &v1beta1.MeterDefinition{},
				lister:       CreateMeterDefinitionV1Beta1Watch(c),
			},
			namespaces: nses,
			Store:      store,
		},
	}
}

var EngineSet = wire.NewSet(
	ProvideEngine,
	dictionary.NewMeterDefinitionDictionary,
	meterdefinition.NewMeterDefinitionStore,
)
