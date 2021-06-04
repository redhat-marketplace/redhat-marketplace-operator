package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/InVisionApp/go-health/v2"
	"github.com/go-logr/logr"
	"github.com/google/wire"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1client "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/dictionary"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/meterdefinition"
	marketplacev1beta1client "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/generated/clientset/versioned/typed/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	pkgtypes "github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/types"
)

type Engine struct {
	store            *meterdefinition.MeterDefinitionStore
	namespaces       pkgtypes.Namespaces
	kubeClient       clientset.Interface
	monitoringClient *monitoringv1client.MonitoringV1Client
	dictionary       *dictionary.MeterDefinitionDictionary
	mktplaceClient   *marketplacev1beta1client.MarketplaceV1beta1Client
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

	for _, runnable := range e.runnables {
		e.log.Info("starting runnable", "runnable", fmt.Sprintf("%T", runnable))
		go runnable.Start(localCtx)
	}

	return e.health.Start()
}

type reflectorConfig struct {
	expectedType runtime.Object
	lister       func(string) cache.ListerWatcher

	startContext context.Context

	cancelFunc context.CancelFunc
}

type ListerRunnable struct {
	reflectorConfig
	namespaces pkgtypes.Namespaces
	store      cache.Store
}

func (p *ListerRunnable) Start(ctx context.Context) error {
	if p.startContext == nil {
		p.startContext = ctx
	}

	var localCtx context.Context
	localCtx, p.cancelFunc = context.WithCancel(p.startContext)

	for _, ns := range p.namespaces {
		localNS := ns
		lister := p.lister(localNS)
		reflector := cache.NewReflector(lister, p.expectedType, p.store, 5*60*time.Second)
		go reflector.Run(localCtx.Done())
	}

	return nil
}

type PVCListerRunnable ListerRunnable

func ProvidePVCLister(
	kubeClient clientset.Interface,
	nses pkgtypes.Namespaces,
	store *meterdefinition.MeterDefinitionStore,
) *PVCListerRunnable {
	return &PVCListerRunnable{
		reflectorConfig: reflectorConfig{
			expectedType: &corev1.PersistentVolumeClaim{},
			lister:       CreatePVCListWatch(kubeClient),
		},
		store:      store,
		namespaces: nses,
	}
}

type PodListerRunnable ListerRunnable

func ProvidePodListerRunnable(
	kubeClient clientset.Interface,
	nses pkgtypes.Namespaces,
	store *meterdefinition.MeterDefinitionStore,
) *PodListerRunnable {
	return &PodListerRunnable{
		reflectorConfig: reflectorConfig{
			expectedType: &corev1.Pod{},
			lister:       CreatePodListWatch(kubeClient),
		},
		namespaces: nses,
		store:      store,
	}
}

type ServiceListerRunnable ListerRunnable

func ProvideServiceListerRunnable(
	kubeClient clientset.Interface,
	nses pkgtypes.Namespaces,
	store *meterdefinition.MeterDefinitionStore,
) *ServiceListerRunnable {
	return &ServiceListerRunnable{
		reflectorConfig: reflectorConfig{
			expectedType: &corev1.Service{},
			lister:       CreateServiceListWatch(kubeClient),
		},
		namespaces: nses,
		store:      store,
	}
}

type ServiceMonitorListerRunnable ListerRunnable

func ProvideServiceMonitorListerRunnable(
	c *monitoringv1client.MonitoringV1Client,
	nses pkgtypes.Namespaces,
	store *meterdefinition.MeterDefinitionStore,
) *ServiceMonitorListerRunnable {
	return &ServiceMonitorListerRunnable{
		reflectorConfig: reflectorConfig{
			expectedType: &monitoringv1.ServiceMonitor{},
			lister:       CreateServiceMonitorListWatch(c),
		},
		namespaces: nses,
		store:      store,
	}
}

type MeterDefinitionListerRunnable ListerRunnable

func ProvideMeterDefinitionListerRunnable(
	nses pkgtypes.Namespaces,
	c *marketplacev1beta1client.MarketplaceV1beta1Client,
	dictionary *dictionary.MeterDefinitionDictionary,
) *MeterDefinitionListerRunnable {
	return &MeterDefinitionListerRunnable{
		reflectorConfig: reflectorConfig{
			expectedType: &v1beta1.MeterDefinition{},
			lister:       CreateMeterDefinitionV1Beta1Watch(c),
		},
		namespaces: nses,
		store:      dictionary,
	}
}

var EngineSet = wire.NewSet(
	ProvideEngine,
	dictionary.NewMeterDefinitionDictionary,
	meterdefinition.NewMeterDefinitionStore,
)
