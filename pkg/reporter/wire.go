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
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/controller"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/managers"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
)

func NewTask(
	ctx context.Context,
	reportName ReportName,
	config *Config,
) (*Task, error) {
	panic(wire.Build(
		reconcileutils.CommandRunnerProviderSet,
		managers.ProvideCachedClientSet,
		wire.FieldsOf(new(*Config), "UploaderTarget"),
		wire.Struct(new(Task), "*"),
		wire.InterfaceValue(new(logr.Logger), logger),
		getClientOptions,
		controller.SchemeDefinitions,
		ProvideUploader,
		wire.Struct(new(managers.CacheIsIndexed)),
	))
}

func NewReporter(
	task *Task,
) (*MarketplaceReporter, error) {
	panic(wire.Build(
		wire.FieldsOf(new(*Task),
			"ReportName", "K8SClient", "Ctx", "Config", "K8SScheme"),
		provideApiClient,
		reconcileutils.CommandRunnerProviderSet,
		wire.InterfaceValue(new(logr.Logger), logger),
		getMarketplaceReport,
		getPrometheusService,
		getMeterDefinitions,
		getMarketplaceConfig,
		ReporterSet,
	))
}
