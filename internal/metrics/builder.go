package metrics

import (
	"context"
	"strings"

	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	"k8s.io/kube-state-metrics/pkg/options"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Builder struct {
	kubeContClient   client.Client
	kubeClient       clientset.Interface
	namespaces       options.NamespaceList
	ctx              context.Context
	enabledResources []string
	shard            int32
	totalShards      int
	cc               reconcileutils.ClientCommandRunner
}

// NewBuilder returns a new builder.
func NewBuilder() *Builder {
	b := &Builder{}
	return b
}

// WithNamespaces sets the namespaces property of a Builder.
func (b *Builder) WithNamespaces(n options.NamespaceList) {
	b.namespaces = n
}

// WithSharding sets the shard and totalShards property of a Builder.
func (b *Builder) WithSharding(shard int32, totalShards int) {
	b.shard = shard
	b.totalShards = totalShards
}

// WithContext sets the ctx property of a Builder.
func (b *Builder) WithContext(ctx context.Context) {
	b.ctx = ctx
}

// WithKubeClient sets the kubeClient property of a Builder.
func (b *Builder) WithKubeClient(c clientset.Interface) {
	b.kubeClient = c
}

// WithKubeControllerClient sets the kubeClient property of a Builder.
func (b *Builder) WithKubeControllerClient(c client.Client) {
	b.kubeContClient = c
}

func (b *Builder) WithClientCommand(cc reconcileutils.ClientCommandRunner) {
	b.cc = cc
}

func (b *Builder) Build() []*MetricsStore {
	stores := []*MetricsStore{}
	activeStoreNames := []string{"pods", "services"}

	klog.Info("Active resources", "resources", strings.Join(activeStoreNames, ","))

	for _, storeName := range activeStoreNames {
		stores = append(stores, availableStores[storeName](b))
	}

	return stores
}

var availableStores = map[string]func(f *Builder) *MetricsStore{
	"pods":    func(b *Builder) *MetricsStore { return b.buildPodStore() },
	"services": func(b *Builder) *MetricsStore { return b.buildServiceStore() },
	// "services": func(b *Builder) cache.Store {
	// 	return b.buildServiceStore()
	// },
}

func resourceExists(name string) bool {
	_, ok := availableStores[name]
	return ok
}

func createPodListWatch(kubeClient clientset.Interface, ns string) cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return kubeClient.CoreV1().Pods(ns).List(context.TODO(), opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return kubeClient.CoreV1().Pods(ns).Watch(context.TODO(), opts)
		},
	}
}

func createServiceListWatch(kubeClient clientset.Interface, ns string) cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return kubeClient.CoreV1().Services(ns).List(context.TODO(), opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return kubeClient.CoreV1().Services(ns).Watch(context.TODO(), opts)
		},
	}
}

func (b *Builder) buildServiceStore() *MetricsStore {
	return b.buildStore(serviceMetricsFamilies, &v1.Service{}, &ServiceMeterDefFetcher{b.cc}, createServiceListWatch)
}

func (b *Builder) buildPodStore() *MetricsStore {
	return b.buildStore(podMetricsFamilies, &v1.Pod{}, &PodMeterDefFetcher{b.cc}, createPodListWatch)
}

func (b *Builder) buildStore(
	metricFamilies []FamilyGenerator,
	expectedType interface{},
	fetcher MeterDefinitionFetcher,
	listWatchFunc func(kubeClient clientset.Interface, ns string) cache.ListerWatcher,
) *MetricsStore {
	composedMetricGenFuncs := ComposeMetricGenFuncs(metricFamilies)
	familyHeaders := ExtractMetricFamilyHeaders(metricFamilies)

	store := NewMetricsStore(
		familyHeaders,
		composedMetricGenFuncs,
		fetcher,
	)
	b.reflectorPerNamespace(expectedType, store, listWatchFunc)

	return store
}

// reflectorPerNamespace creates a Kubernetes client-go reflector with the given
// listWatchFunc for each given namespace and registers it with the given store.
func (b *Builder) reflectorPerNamespace(
	expectedType interface{},
	store cache.Store,
	listWatchFunc func(kubeClient clientset.Interface, ns string) cache.ListerWatcher,
) {
	for _, ns := range b.namespaces {
		lw := listWatchFunc(b.kubeClient, ns)
		reflector := cache.NewReflector(lw, expectedType, store, 0)
		go reflector.Run(b.ctx.Done())
	}
}

func ComposeMetricGenFuncs(familyGens []FamilyGenerator) func(interface{}, []*marketplacev1alpha1.MeterDefinition) []FamilyByteSlicer {
	return func(obj interface{}, meterDefinitions []*marketplacev1alpha1.MeterDefinition) []FamilyByteSlicer {
		families := make([]FamilyByteSlicer, len(familyGens))

		for i, gen := range familyGens {
			family := gen.GenerateMeterFunc(obj, meterDefinitions)
			family.Name = gen.Name
			families[i] = family
		}

		return families
	}
}

// ExtractMetricFamilyHeaders takes in a slice of FamilyGenerator metrics and
// returns the extracted headers.
func ExtractMetricFamilyHeaders(families []FamilyGenerator) []string {
	headers := make([]string, len(families))

	for i, f := range families {
		headers[i] = f.generateHeader()
	}

	return headers
}
