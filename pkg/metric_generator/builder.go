package metric_generator

import (
	"context"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	generator "k8s.io/kube-state-metrics/pkg/metric"
	metricsstore "k8s.io/kube-state-metrics/pkg/metrics_store"
	"k8s.io/kube-state-metrics/pkg/options"
)

type Builder struct {
	kubeClient       client.Client
	namespaces       options.NamespaceList
	ctx              context.Context
	enabledResources []string
	shard            int32
	totalShards      int

	collector *Collector
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
func (b *Builder) WithKubeClient(c client.Client) {
	b.kubeClient = c
}

func (b *Builder) WithCollector(c *Collector) {
	b.collector = c
}

func (b *Builder) Build() []*metricsstore.MetricsStore {
	stores := []*metricsstore.MetricsStore{}
	activeStoreNames := []string{"pods"}

	klog.Info("Active resources", "resources", strings.Join(activeStoreNames, ","))

	for _, storeName := range activeStoreNames {
		stores = append(stores, availableStores[storeName](b))
	}

	return stores
}

var availableStores = map[string]func(f *Builder) *metricsstore.MetricsStore{
	"pods": func(b *Builder) *metricsstore.MetricsStore {
		store := b.buildPodStore()
		podChan := make(chan *v1.Pod)
		b.collector.RegisterPodListener(podChan)

		go func() {
			for pod := range podChan {
				store.Add(pod)
			}
		}()

		return store
	},
	// "services": func(b *Builder) cache.Store {
	// 	return b.buildServiceStore()
	// },
}

func resourceExists(name string) bool {
	_, ok := availableStores[name]
	return ok
}

// func (b *Builder) buildServiceStore() cache.Store {
// 	return b.buildStoreFunc(serviceMetricsFamilies, &v1.Service{}, createServiceListWatch)
// }

func (b *Builder) buildPodStore() *metricsstore.MetricsStore {
	return b.buildStore(podMetricsFamilies, &v1.Pod{})
}

func (b *Builder) buildStore(
	metricFamilies []generator.FamilyGenerator,
	expectedType interface{},
) *metricsstore.MetricsStore {
	composedMetricGenFuncs := generator.ComposeMetricGenFuncs(metricFamilies)
	familyHeaders := generator.ExtractMetricFamilyHeaders(metricFamilies)

	store := metricsstore.NewMetricsStore(
		familyHeaders,
		composedMetricGenFuncs,
	)

	return store
}
