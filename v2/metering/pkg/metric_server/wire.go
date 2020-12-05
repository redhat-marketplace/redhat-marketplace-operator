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

// +build wireinject

package metric_server

import (
	monitoringv1client "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"
	"github.com/go-logr/logr"
	"github.com/google/wire"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/client"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/client"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/controller"
	marketplacev1alpha1client "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/generated/clientset/versioned/typed/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/managers"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/metering/pkg/meter_definition"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
)

func NewServer(
	opts *Options,
) (*Service, error) {
	panic(wire.Build(
		managers.ProvideCachedClientSet,
		getClientOptions,
		controller.SchemeDefinitions,
		reconcileutils.CommandRunnerProviderSet,
		ConvertOptions,
		wire.Struct(new(Service), "*"),
		wire.InterfaceValue(new(logr.Logger), log),
		provideRegistry,
		meter_definition.NewMeterDefinitionStoreBuilder,
		meter_definition.NewStatusProcessor,
		meter_definition.NewServiceProcessor,
		marketplacev1alpha1client.NewForConfig,
		monitoringv1client.NewForConfig,
		provideContext,
		rhmclient.NewFindOwnerHelper,
		client.NewDynamicClient,
		addIndex,
	))
}
