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

//go:build wireinject
// +build wireinject

package reporter

import (
	"context"

	"github.com/google/wire"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/dataservice"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/client"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/managers"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/prometheus"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewTask(
	ctx context.Context,
	reportName ReportName,
	taskConfig *Config,
) (TaskRun, error) {
	panic(wire.Build(
		wire.FieldsOf(new(*Config), "K8sRestConfig"),
		managers.ProvideSimpleClientSet,
		wire.Struct(new(Task), "*"),
		wire.Value(logger),
		ProvideUploaders,
		ProvideUploader,
		provideScheme,
		wire.Bind(new(TaskRun), new(*Task)),
		wire.Bind(new(client.Client), new(rhmclient.SimpleClient)),
	))
}

func NewEventBroadcaster(
	erConfig *Config,
) (record.EventBroadcaster, func(), error) {
	panic(wire.Build(
		wire.FieldsOf(new(*Config), "K8sRestConfig"),
		managers.ProvideSimpleClientSet,
		provideReporterEventBroadcaster,
	))
}

func NewReporter(
	ctx context.Context,
	task *Task,
) (*MarketplaceReporter, error) {
	panic(wire.Build(
		wire.FieldsOf(new(*Task),
			"ReportName", "K8SClient", "Config"),
		providePrometheusSetup,
		prometheus.NewPrometheusAPIForReporter,
		wire.Value(logger),
		getMarketplaceReport,
		getPrometheusService,
		getPrometheusPort,
		getMarketplaceConfig,
		getMeterDefinitionReferences,
		getK8sInfrastructureResources,
		ProvideWriter,
		ProvideDataBuilder,
		ReporterSet,
		wire.Bind(new(client.Client), new(rhmclient.SimpleClient)),
	))
}

func NewUploadTask(
	ctx context.Context,
	config *Config,
	namespace Namespace,
) (UploadRun, error) {
	panic(wire.Build(
		wire.FieldsOf(new(*Config), "K8sRestConfig"),
		managers.ProvideSimpleClientSet,
		provideScheme,
		wire.Bind(new(client.Client), new(rhmclient.SimpleClient)),
		wire.Bind(new(dataservice.FileStorage), new(*dataservice.DataService)),
		ProvideUploaders,
		dataservice.NewDataService,
		provideDataServiceConfig,
		provideGRPCDialOptions,
		wire.Struct(new(UploadTask), "*"),
		wire.Value(logger),
		wire.Bind(new(UploadRun), new(*UploadTask)),
	))
}

func NewReconcileTask(
	ctx context.Context,
	config *Config,
	broadcaster record.EventBroadcaster,
	namespace Namespace,
	newReportTask func(ctx context.Context, reportName ReportName, taskConfig *Config) (TaskRun, error),
	newUploadTask func(ctx context.Context, config *Config, namespace Namespace) (UploadRun, error),
) (*ReconcileTask, error) {
	panic(wire.Build(
		wire.FieldsOf(new(*Config), "K8sRestConfig"),
		managers.ProvideSimpleClientSet,
		wire.Struct(new(ReconcileTask), "*"),
		provideScheme,
		provideReporterEventRecorder,
		wire.Bind(new(client.Client), new(rhmclient.SimpleClient)),
		wire.Value(logger),
	))
}
