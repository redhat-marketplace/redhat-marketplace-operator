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

package reporter

import (
	"context"
	"fmt"
	"path/filepath"

	"emperror.dev/errors"
	"github.com/google/uuid"
	"github.com/gotidy/ptr"
	"github.com/prometheus/common/log"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/managers"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	opsrcv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	marketplaceredhatcomv1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/prometheus"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

type Task struct {
	ReportName ReportName

	CC        ClientCommandRunner
	K8SClient client.Client
	Ctx       context.Context
	Config    *Config
	K8SScheme *runtime.Scheme
	Uploader
}

func (r *Task) Run() error {
	logger.Info("task run start")
	logger.Info("creating reporter job")
	reporter, err := NewReporter(r)

	if err != nil {
		return err
	}

	logger.Info("starting collection")
	metrics, errorList, err := reporter.CollectMetrics(r.Ctx)

	if err != nil {
		logger.Error(err, "error collecting metrics")
		return err
	}

	reportID := uuid.New()

	logger.Info("writing report", "reportID", reportID)

	files, err := reporter.WriteReport(
		reportID,
		metrics)

	if err != nil {
		return errors.Wrap(err, "error writing report")
	}

	dirpath := filepath.Dir(files[0])
	fileName := fmt.Sprintf("%s/../upload-%s.tar.gz", dirpath, reportID.String())
	err = TargzFolder(dirpath, fileName)

	logger.Info("tarring", "outputfile", fileName)

	if r.Config.Upload {
		err = r.Uploader.UploadFile(fileName)

		if err != nil {
			return errors.Wrap(err, "error uploading file")
		}

		logger.Info("uploaded metrics", "metrics", len(metrics))
	}

	report := &marketplacev1alpha1.MeterReport{}
	err = utils.Retry(func() error {
		result, _ := r.CC.Do(
			r.Ctx,
			HandleResult(
				GetAction(types.NamespacedName(r.ReportName), report),
				OnContinue(Call(func() (ClientAction, error) {
					report.Status.MetricUploadCount = ptr.Int(len(metrics))

					report.Status.QueryErrorList = []string{}

					for _, err := range errorList {
						report.Status.QueryErrorList = append(report.Status.QueryErrorList, err.Error())
					}

					return UpdateAction(report), nil
				})),
			),
		)

		if result.Is(Error) {
			return result
		}

		return nil
	}, 3)

	if err != nil {
		log.Error(err, "failed to update report")
	}

	return nil
}

func providePrometheusSetup(config *Config,report *marketplacev1alpha1.MeterReport,promService *corev1.Service)*PrometheusAPISetup{
	return &PrometheusAPISetup{
		Report: report,
		PromService: promService,
		CertFilePath: config.CaFile,
		TokenFilePath:  config.TokenFile,
		RunLocal: config.Local,
	}
}

func getClientOptions() managers.ClientOptions {
	return managers.ClientOptions{
		Namespace:    "",
		DryRunClient: false,
	}
}

func getMarketplaceConfig(
	ctx context.Context,
	cc ClientCommandRunner,
) (config *marketplacev1alpha1.MarketplaceConfig, returnErr error) {
	config = &marketplacev1alpha1.MarketplaceConfig{}

	if result, _ := cc.Do(ctx,
		GetAction(
			types.NamespacedName{Namespace: "openshift-redhat-marketplace", Name: utils.MARKETPLACECONFIG_NAME}, config,
		)); !result.Is(Continue) {
		returnErr = errors.Wrap(result, "failed to get mkplc config")
	}

	logger.Info("retrieved meter report")
	return
}

func getMarketplaceReport(
	ctx context.Context,
	cc ClientCommandRunner,
	reportName ReportName,
) (report *marketplacev1alpha1.MeterReport, returnErr error) {
	report = &marketplacev1alpha1.MeterReport{}

	if result, _ := cc.Do(ctx, GetAction(types.NamespacedName(reportName), report)); !result.Is(Continue) {
		returnErr = errors.Wrap(result, "failed to get report")
	}

	logger.Info("retrieved meter report")
	return
}

func getPrometheusService(
	ctx context.Context,
	report *marketplacev1alpha1.MeterReport,
	cc ClientCommandRunner,
) (service *corev1.Service, returnErr error) {
	service = &corev1.Service{}

	if report.Spec.PrometheusService == nil {
		returnErr = errors.New("cannot retrieve service as the report doesn't have a value for it")
		return
	}

	name := types.NamespacedName{
		Name:      report.Spec.PrometheusService.Name,
		Namespace: report.Spec.PrometheusService.Namespace,
	}

	if result, _ := cc.Do(ctx, GetAction(name, service)); !result.Is(Continue) {
		returnErr = errors.Wrap(result, "failed to get report")
	}

	logger.Info("retrieved prometheus service")
	return
}

func getMeterDefinitions(
	ctx context.Context,
	report *marketplacev1alpha1.MeterReport,
	cc ClientCommandRunner,
) ([]marketplacev1alpha1.MeterDefinition, error) {
	defs := &marketplacev1alpha1.MeterDefinitionList{}

	if len(report.Spec.MeterDefinitions) > 0 {
		return report.Spec.MeterDefinitions, nil
	}

	result, _ := cc.Do(ctx,
		HandleResult(
			ListAction(defs, client.InNamespace("")),
			OnContinue(Call(func() (ClientAction, error) {
				for _, item := range defs.Items {
					item.Status = marketplacev1alpha1.MeterDefinitionStatus{}
				}

				report.Spec.MeterDefinitions = defs.Items

				return UpdateAction(report), nil
			})),
		),
	)

	if result.Is(Error) {
		return nil, errors.Wrap(result, "failed to get meterdefs")
	}

	if result.Is(NotFound) {
		return []marketplacev1alpha1.MeterDefinition{}, nil
	}

	return defs.Items, nil
}

func provideScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(marketplaceredhatcomv1alpha1.AddToScheme(scheme))
	utilruntime.Must(openshiftconfigv1.AddToScheme(scheme))
	utilruntime.Must(olmv1.AddToScheme(scheme))
	utilruntime.Must(opsrcv1.AddToScheme(scheme))
	utilruntime.Must(olmv1alpha1.AddToScheme(scheme))
	utilruntime.Must(monitoringv1.AddToScheme(scheme))
	return scheme
}
