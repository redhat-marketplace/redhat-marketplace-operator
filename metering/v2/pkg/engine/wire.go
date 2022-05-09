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
	monitoringv1client "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/internal/metrics"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/processors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/types"
	marketplacev1beta1client "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/generated/clientset/versioned/typed/marketplace/v1beta1"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/client"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/managers"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func NewEngine(
	ctx context.Context,
	namespaces types.Namespaces,
	scheme *runtime.Scheme,
	clientOptions managers.ClientOptions,
	log logr.Logger,
	prometheusData *metrics.PrometheusData,
	statusFlushDuration processors.StatusFlushDuration,
) (*Engine, error) {
	panic(wire.Build(
		config.GetConfig,
		managers.ProvideMetadataClientSet,
		managers.AddIndices,
		RunnablesSet,
		EngineSet,
		marketplacev1beta1client.NewForConfig,
		monitoringv1client.NewForConfig,
		rhmclient.NewFindOwnerHelper,
		rhmclient.NewMetadataClient,
	))
}
