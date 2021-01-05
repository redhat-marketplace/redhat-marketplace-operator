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

package metrics

import (
	"context"
	"reflect"
	"strings"

	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/meter_definition"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	v1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
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
	meterDefStores   meter_definition.MeterDefinitionStores
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

func (b *Builder) WithMeterDefinitionStores(stores meter_definition.MeterDefinitionStores) {
	b.meterDefStores = stores
}

func (b *Builder) Build() []*MetricsStore {
	stores := []*MetricsStore{}
	activeStoreNames := []string{"pods", "services", "persistentvolumeclaims", "meterdefinitions"}

	klog.Info("Active resources", "resources", strings.Join(activeStoreNames, ","))

	for _, storeName := range activeStoreNames {
		store := availableStores[storeName](b)
		stores = append(stores, store)
	}

	return stores
}

var availableStores = map[string]func(f *Builder) *MetricsStore{
	"pods":                   func(b *Builder) *MetricsStore { return b.buildPodStore() },
	"services":               func(b *Builder) *MetricsStore { return b.buildServiceStore() },
	"persistentvolumeclaims": func(b *Builder) *MetricsStore { return b.buildPVCStore() },
	"meterdefinitions":       func(b *Builder) *MetricsStore { return b.buildMeterDefinitionStore() },
}

var (
	serviceType         reflect.Type = reflect.TypeOf(&v1.Service{})
	podType                          = reflect.TypeOf(&v1.Pod{})
	persistentVolType                = reflect.TypeOf(&v1.PersistentVolumeClaim{})
	meterDefinitionType              = reflect.TypeOf(&marketplacev1alpha1.MeterDefinition{})
)

func (b *Builder) buildServiceStore() *MetricsStore {
	return b.buildStore(
		serviceMetricsFamilies,
		serviceType,
		&meterDefFetcher{b.cc, b.meterDefStores[meter_definition.ServiceStore]},
		b.meterDefStores[meter_definition.ServiceStore],
	)
}

func (b *Builder) buildPodStore() *MetricsStore {
	return b.buildStore(
		podMetricsFamilies,
		podType,
		&meterDefFetcher{b.cc, b.meterDefStores[meter_definition.PodStore]},
		b.meterDefStores[meter_definition.PodStore],
	)
}

func (b *Builder) buildPVCStore() *MetricsStore {
	return b.buildStore(
		pvcMetricsFamilies,
		persistentVolType,
		&meterDefFetcher{b.cc, b.meterDefStores[meter_definition.PersistentVolumeStore]},
		b.meterDefStores[meter_definition.PersistentVolumeStore],
	)
}

func (b *Builder) buildMeterDefinitionStore() *MetricsStore {
	return b.buildStore(
		meterDefinitionMetricsFamilies,
		meterDefinitionType,
		emptyFetcher,
		b.meterDefStores[meter_definition.PodStore],
	)
}

func (b *Builder) buildStore(
	metricFamilies []FamilyGenerator,
	expectedType reflect.Type,
	meterDefFetcher MeterDefinitionFetcher,
	meterStore *meter_definition.MeterDefinitionStore,
) *MetricsStore {
	composedMetricGenFuncs := ComposeMetricGenFuncs(metricFamilies)
	familyHeaders := ExtractMetricFamilyHeaders(metricFamilies)

	return NewMetricsStore(
		familyHeaders,
		composedMetricGenFuncs,
		meterStore,
		meterDefFetcher,
		expectedType,
	)
}

func ComposeMetricGenFuncs(familyGens []FamilyGenerator) func(interface{}, []*marketplacev1beta1.MeterDefinition) []FamilyByteSlicer {
	return func(obj interface{}, meterDefinitions []*marketplacev1beta1.MeterDefinition) []FamilyByteSlicer {
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
