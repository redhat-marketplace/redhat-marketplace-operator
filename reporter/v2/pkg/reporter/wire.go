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

package reporter

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/google/wire"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/client"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/managers"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/prometheus"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

func NewTask(
	ctx context.Context,
	reportName ReportName,
	taskConfig *Config,
) (*Task, error) {
	panic(wire.Build(
		reconcileutils.CommandRunnerProviderSet,
		managers.ProvideSimpleClientSet,
		wire.Struct(new(Task), "*"),
		wire.InterfaceValue(new(logr.Logger), logger),
		ProvideUploaders,
		provideScheme,
		wire.Bind(new(client.Client), new(rhmclient.SimpleClient)),
		kconfig.GetConfig,
	))
}

func NewReporter(
	task *Task,
) (*MarketplaceReporter, error) {
	panic(wire.Build(
		wire.FieldsOf(new(*Task),
			"ReportName", "K8SClient", "Ctx", "Config", "K8SScheme"),
		providePrometheusSetup,
		prometheus.NewPrometheusAPIForReporter,
		reconcileutils.CommandRunnerProviderSet,
		wire.InterfaceValue(new(logr.Logger), logger),
		getMarketplaceReport,
		getPrometheusService,
		getPrometheusPort,
		getMarketplaceConfig,
		getMeterDefinitionReferences,
		ProvideWriter,
		ProvideDataBuilder,
		ReporterSet,
		wire.Bind(new(client.Client), new(rhmclient.SimpleClient)),
	))
}

func NewUploadTask(
	ctx context.Context,
	config *Config,
) (*UploadTask, error) {
	panic(wire.Build(
		reconcileutils.CommandRunnerProviderSet,
		managers.ProvideSimpleClientSet,
		kconfig.GetConfig,
		wire.Struct(new(UploadTask), "*"),
		wire.InterfaceValue(new(logr.Logger), logger),
		ProvideDownloader,
		ProvideUploaders,
		ProvideAdmin,
		provideScheme,
		wire.Bind(new(client.Client), new(rhmclient.SimpleClient)),
	))
}

func NewReconcileTask(
	ctx context.Context,
	config *Config,
	namespace Namespace,
) (*ReconcileTask, error) {
	panic(wire.Build(
		managers.ProvideSimpleClientSet,
		kconfig.GetConfig,
		wire.Struct(new(ReconcileTask), "*"),
		provideScheme,
		wire.Bind(new(client.Client), new(rhmclient.SimpleClient)),
	))
}
