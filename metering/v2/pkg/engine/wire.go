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

// +build wireinject

package engine

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/google/wire"
	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	olmv1alpha1client "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned/typed/operators/v1alpha1"
	monitoringv1client "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/internal/metrics"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/dictionary"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/types"
	marketplacev1alpha1client "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/generated/clientset/versioned/typed/marketplace/v1alpha1"
	marketplacev1beta1client "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/generated/clientset/versioned/typed/marketplace/v1beta1"

	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/client"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/client"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/managers"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

func NewEngine(
	ctx context.Context,
	namespaces types.Namespaces,
	scheme *runtime.Scheme,
	clientOptions managers.ClientOptions,
	k8sRestConfig *rest.Config,
	log logr.Logger,
	prometheusData *metrics.PrometheusData,
) (*Engine, error) {
	panic(wire.Build(
		managers.ProvideCachedClientSet,
		dictionary.ProvideMeterDefinitionList,
		RunnablesSet,
		EngineSet,
		marketplacev1beta1client.NewForConfig,
		monitoringv1client.NewForConfig,
		rhmclient.NewFindOwnerHelper,
		client.NewDynamicClient,
		managers.AddIndices,
	))
}

func NewRazeeEngine(
	ctx context.Context,
	namespaces types.Namespaces,
	scheme *runtime.Scheme,
	clientOptions managers.ClientOptions,
	k8sRestConfig *rest.Config,
	log logr.Logger,
) (*RazeeEngine, error) {
	panic(wire.Build(
		discovery.NewDiscoveryClientForConfig,
		managers.ProvideCachedClientSet,
		RazeeRunnablesSet,
		RazeeEngineSet,
		marketplacev1alpha1client.NewForConfig,
		olmv1alpha1client.NewForConfig,
		configv1client.NewForConfig,
		rhmclient.NewFindOwnerHelper,
		client.NewDynamicClient,
		wire.Struct(new(managers.CacheIsIndexed)),
	))
}
