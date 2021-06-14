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
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/managers"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	opsrcv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/client"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/prometheus"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/retry"
)

type Task struct {
	ReportName ReportName

	CC        ClientCommandRunner
	K8SClient rhmclient.SimpleClient
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
	metrics, errorList, warningList, _ := reporter.CollectMetrics(r.Ctx)

	for _, err := range warningList {
		details := append(
			[]interface{}{"cause", errors.Cause(err)},
			errors.GetDetails(err)...)
		logger.Info(fmt.Sprintf("warning: %v", errors.Cause(err)), details...)
	}

	reportID := uuid.MustParse(reporter.report.Spec.ReportUUID)

	logger.Info("writing report", "reportID", r.ReportName)

	files, err := reporter.WriteReport(reportID, metrics)

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

		logger.Info("uploaded metrics", "metricsLength", len(metrics))
	}

	report := &marketplacev1alpha1.MeterReport{}
	err = utils.Retry(func() error {
		result, _ := r.CC.Do(
			r.Ctx,
			HandleResult(
				GetAction(types.NamespacedName(r.ReportName), report),
				OnContinue(Call(func() (ClientAction, error) {
					report.Status.MetricUploadCount = ptr.Int(len(metrics))
					report.Status.Errors = make([]marketplacev1alpha1.ErrorDetails, 0, len(errorList))
					report.Status.Warnings = make([]marketplacev1alpha1.ErrorDetails, 0, len(warningList))

					for _, err := range errorList {
						report.Status.Errors = append(report.Status.Errors,
							(marketplacev1alpha1.ErrorDetails{}).FromError(err),
						)
					}

					for _, err := range warningList {
						report.Status.Warnings = append(report.Status.Warnings,
							(marketplacev1alpha1.ErrorDetails{}).FromError(err),
						)
					}

					return UpdateAction(report, UpdateStatusOnly(true)), nil
				})),
			),
		)

		if result.Is(Error) {
			return result
		}

		if len(errorList) != 0 {
			return errors.Combine(errorList...)
		}

		return nil
	}, 3)

	if err != nil {
		log.Error(err, "failed to update report status")
	}

	return nil
}

func providePrometheusSetup(config *Config, report *marketplacev1alpha1.MeterReport, promService *corev1.Service) *PrometheusAPISetup {
	return &PrometheusAPISetup{
		Report:        report,
		PromService:   promService,
		CertFilePath:  config.CaFile,
		TokenFilePath: config.TokenFile,
		RunLocal:      config.Local,
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

	logger.Info("retrieved mkplc config")
	return
}

func getMarketplaceReport(
	ctx context.Context,
	cc ClientCommandRunner,
	reportName ReportName,
) (report *marketplacev1alpha1.MeterReport, returnErr error) {
	report = &marketplacev1alpha1.MeterReport{}

	returnErr = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if result, err := cc.Do(ctx, GetAction(types.NamespacedName(reportName), report)); !result.Is(Continue) {
			return err
		}

		if report.Spec.ReportUUID == "" {
			report.Spec.ReportUUID = uuid.New().String()

			if result, err := cc.Exec(ctx, UpdateAction(report)); !result.Is(Continue) {
				return err
			}
		}

		return nil
	})

	if returnErr != nil {
		return
	}

	logger.Info("retrieved meter report", "name", reportName)
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

type MeterDefinitionReferences = []marketplacev1beta1.MeterDefinitionReference

func getMeterDefinitionReferences(
	ctx context.Context,
	report *marketplacev1alpha1.MeterReport,
	cc ClientCommandRunner,
) (MeterDefinitionReferences, error) {
	defs := []marketplacev1beta1.MeterDefinitionReference{}

	if len(report.Spec.MeterDefinitionReferences) > 0 {
		update := false

		for _, ref := range report.Spec.MeterDefinitionReferences {
			if ref.Spec == nil {
				update = true

				mdef := marketplacev1beta1.MeterDefinition{}
				result, _ := cc.Exec(ctx, GetAction(types.NamespacedName{
					Name: ref.Name, Namespace: ref.Namespace,
				}, &mdef))

				if result.Is(Error) {
					return defs, result.Err
				}

				if result.Is(Continue) {
					update = true
					ref.UID = mdef.UID
					ref.Spec = &mdef.Spec
				}
			}

			defs = append(defs, ref)
		}

		if update {
			result, _ := cc.Exec(ctx, UpdateAction(report))
			if result.Is(Error) {
				return defs, result.Err
			}
		}

		return defs, nil
	}

	return defs, nil
}

func provideScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(marketplacev1alpha1.AddToScheme(scheme))
	utilruntime.Must(marketplacev1beta1.AddToScheme(scheme))
	utilruntime.Must(openshiftconfigv1.AddToScheme(scheme))
	utilruntime.Must(olmv1.AddToScheme(scheme))
	utilruntime.Must(opsrcv1.AddToScheme(scheme))
	utilruntime.Must(olmv1alpha1.AddToScheme(scheme))
	utilruntime.Must(monitoringv1.AddToScheme(scheme))
	return scheme
}
