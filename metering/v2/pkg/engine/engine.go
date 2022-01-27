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
	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	olmv1alpha1client "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned/typed/operators/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1client "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"
	marketplacev1alpha1client "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/generated/clientset/versioned/typed/marketplace/v1alpha1"
	marketplacev1beta1client "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/generated/clientset/versioned/typed/marketplace/v1beta1"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/internal/metrics"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/dictionary"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/meterdefinition"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/razee"
	pkgtypes "github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/managers"
	appsv1 "k8s.io/api/apps/v1"
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

type RazeeEngine struct {
	store      *razee.RazeeStore
	namespaces pkgtypes.Namespaces
	kubeClient clientset.Interface
	runnables  Runnables
	log        logr.Logger
	health     *health.Health

	mainContext  *context.Context
	localContext *context.Context
	cancelFunc   context.CancelFunc
}

func ProvideRazeeEngine(
	cache managers.CacheIsStarted,
	store *razee.RazeeStore,
	namespaces pkgtypes.Namespaces,
	log logr.Logger,
	kubeClient clientset.Interface,
	runnables Runnables,
) *RazeeEngine {
	h := health.New()
	return &RazeeEngine{
		store:      store,
		log:        log,
		namespaces: namespaces,
		kubeClient: kubeClient,
		runnables:  runnables,
		health:     h,
	}
}

func (e *RazeeEngine) Start(ctx context.Context) error {
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

func ProvideMeterDefinitionSeenStoreRunnable(
	kubeClient clientset.Interface,
	nses pkgtypes.Namespaces,
	c *marketplacev1beta1client.MarketplaceV1beta1Client,
	store dictionary.MeterDefinitionsSeenStore,
	log logr.Logger,
) *MeterDefinitionSeenStoreRunnable {
	return &MeterDefinitionSeenStoreRunnable{
		StoreRunnable: StoreRunnable{
			Store:      store,
			ResyncTime: 0,
			Reflectors: []Runnable{
				provideMeterDefinitionListerRunnable(nses, c, store),
			},
			log: log.WithName("meterdefseenstore"),
		},
	}
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

type ObjectsSeenStoreRunnable struct {
	StoreRunnable
}

func ProvideObjectsSeenStoreRunnable(
	kubeClient clientset.Interface,
	nses pkgtypes.Namespaces,
	store meterdefinition.ObjectsSeenStore,
	c *monitoringv1client.MonitoringV1Client,
	log logr.Logger,
) *ObjectsSeenStoreRunnable {
	return &ObjectsSeenStoreRunnable{
		StoreRunnable: StoreRunnable{
			Store:      store,
			ResyncTime: 0, //1*60*time.Second,
			log:        log.WithName("objectsseen"),
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

type RazeeStoreRunnable struct {
	StoreRunnable
}

func ProvideRazeeStoreRunnable(
	kubeClient clientset.Interface,
	olmv1alpha1Client *olmv1alpha1client.OperatorsV1alpha1Client,
	configv1Client *configv1client.ConfigV1Client,
	marketplacev1alpha1Client *marketplacev1alpha1client.MarketplaceV1alpha1Client,
	nses pkgtypes.Namespaces,
	store razee.RazeeStores,
	log logr.Logger,
) *RazeeStoreRunnable {
	return &RazeeStoreRunnable{
		StoreRunnable: StoreRunnable{
			Store:      store,
			ResyncTime: 30 * time.Second,
			log:        log.WithName("razee"),
			Reflectors: []Runnable{
				provideNodeListerRunnable(kubeClient, store),
				provideNamespaceListerRunnable(kubeClient, store),
				provideDeploymentListerRunnable(kubeClient, nses, store),
				provideClusterServiceVersionListerRunnable(olmv1alpha1Client, nses, store),
				provideClusterVersionListerRunnable(configv1Client, store),
				provideConsoleListerRunnable(configv1Client, store),
				provideInfrastructureListerRunnable(configv1Client, store),
				provideMarketplaceConfigListerRunnable(marketplacev1alpha1Client, nses, store),
			},
		},
	}
}

var (
	clusterScoped pkgtypes.Namespaces = []string{""}
)

type NodeListerRunnable struct {
	ListerRunnable
}

func provideNodeListerRunnable(
	kubeClient clientset.Interface,
	store cache.Store,
) *NodeListerRunnable {
	return &NodeListerRunnable{
		ListerRunnable: ListerRunnable{
			reflectorConfig: reflectorConfig{
				expectedType: &corev1.Node{},
				lister:       CreateNodeListWatch(kubeClient),
			},
			Store:      store,
			namespaces: clusterScoped,
		},
	}
}

type NamespaceListerRunnable struct {
	ListerRunnable
}

func provideNamespaceListerRunnable(
	kubeClient clientset.Interface,
	store cache.Store,
) *NamespaceListerRunnable {
	return &NamespaceListerRunnable{
		ListerRunnable: ListerRunnable{
			reflectorConfig: reflectorConfig{
				expectedType: &corev1.Namespace{},
				lister:       CreateNamespaceListWatch(kubeClient),
			},
			Store:      store,
			namespaces: clusterScoped,
		},
	}
}

type DeploymentListerRunnable struct {
	ListerRunnable
}

func provideDeploymentListerRunnable(
	kubeClient clientset.Interface,
	nses pkgtypes.Namespaces,
	store cache.Store,
) *DeploymentListerRunnable {
	return &DeploymentListerRunnable{
		ListerRunnable: ListerRunnable{
			reflectorConfig: reflectorConfig{
				expectedType: &appsv1.Deployment{},
				lister:       CreateDeploymentListWatch(kubeClient),
			},
			namespaces: nses,
			Store:      store,
		},
	}
}

type ClusterServiceVersionListerRunnable struct {
	ListerRunnable
}

func provideClusterServiceVersionListerRunnable(
	c *olmv1alpha1client.OperatorsV1alpha1Client,
	nses pkgtypes.Namespaces,
	store cache.Store,
) *ClusterServiceVersionListerRunnable {
	return &ClusterServiceVersionListerRunnable{
		ListerRunnable: ListerRunnable{
			reflectorConfig: reflectorConfig{
				expectedType: &olmv1alpha1.ClusterServiceVersion{},
				lister:       CreateClusterServiceVersionListWatch(c),
			},
			Store:      store,
			namespaces: nses,
		},
	}
}

type ClusterVersionListerRunnable struct {
	ListerRunnable
}

func provideClusterVersionListerRunnable(
	c *configv1client.ConfigV1Client,
	store cache.Store,
) *ClusterVersionListerRunnable {
	return &ClusterVersionListerRunnable{
		ListerRunnable: ListerRunnable{
			reflectorConfig: reflectorConfig{
				expectedType: &openshiftconfigv1.ClusterVersion{},
				lister:       CreateClusterVersionListWatch(c),
			},
			Store:      store,
			namespaces: clusterScoped,
		},
	}
}

type ConsoleListerRunnable struct {
	ListerRunnable
}

func provideConsoleListerRunnable(
	c *configv1client.ConfigV1Client,
	store cache.Store,
) *ConsoleListerRunnable {
	return &ConsoleListerRunnable{
		ListerRunnable: ListerRunnable{
			reflectorConfig: reflectorConfig{
				expectedType: &openshiftconfigv1.Console{},
				lister:       CreateConsoleListWatch(c),
			},
			Store:      store,
			namespaces: clusterScoped,
		},
	}
}

type InfrastructureListerRunnable struct {
	ListerRunnable
}

func provideInfrastructureListerRunnable(
	c *configv1client.ConfigV1Client,
	store cache.Store,
) *InfrastructureListerRunnable {
	return &InfrastructureListerRunnable{
		ListerRunnable: ListerRunnable{
			reflectorConfig: reflectorConfig{
				expectedType: &openshiftconfigv1.Infrastructure{},
				lister:       CreateInfrastructureListWatch(c),
			},
			Store:      store,
			namespaces: clusterScoped,
		},
	}
}

type MarketplaceConfigListerRunnable struct {
	ListerRunnable
}

func provideMarketplaceConfigListerRunnable(
	c *marketplacev1alpha1client.MarketplaceV1alpha1Client,
	nses pkgtypes.Namespaces,
	store cache.Store,
) *MarketplaceConfigListerRunnable {
	return &MarketplaceConfigListerRunnable{
		ListerRunnable: ListerRunnable{
			reflectorConfig: reflectorConfig{
				expectedType: &v1alpha1.MarketplaceConfig{},
				lister:       CreateMarketplaceConfigV1Alpha1Watch(c),
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
	meterdefinition.NewObjectsSeenStore,
	dictionary.NewMeterDefinitionsSeenStore,
)

var RazeeEngineSet = wire.NewSet(
	ProvideRazeeEngine,
	razee.NewRazeeStore,
	wire.FieldsOf(new(razee.RazeeStoreGroup), "Store", "Stores"),
)
