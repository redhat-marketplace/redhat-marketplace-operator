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
	"os"
	"path/filepath"
	"reflect"
	"time"

	"emperror.dev/errors"
	"github.com/google/uuid"
	"github.com/gotidy/ptr"
	"github.com/prometheus/common/log"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/dataservice"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/uploaders"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/managers"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	opsrcv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/client"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/prometheus"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
)

type TaskRun interface {
	Run(ctx context.Context) error
}

type Task struct {
	ReportName ReportName

	CC        ClientCommandRunner
	K8SClient rhmclient.SimpleClient
	Config    *Config
	K8SScheme *runtime.Scheme
	uploaders.Uploader
}

func (r *Task) Run(ctx context.Context) error {
	logger.Info("task run start")
	logger.Info("creating reporter job")

	return r.report(ctx)
}

func (r *Task) report(ctx context.Context) error {
	reporter, err := NewReporter(ctx, r)

	if err != nil {
		return err
	}

	logger.Info("starting collection")
	metrics, errorList, warningList, _ := reporter.CollectMetrics(ctx)

	for _, err := range warningList {
		details := append(
			[]interface{}{"cause", errors.Cause(err)},
			errors.GetDetails(err)...)
		logger.Info(fmt.Sprintf("warning: %v", err.Error()), details...)
	}

	for _, err := range errorList {
		details := append(
			[]interface{}{"cause", errors.Cause(err)},
			errors.GetDetails(err)...)
		logger.Info(fmt.Sprintf("error: %v", err), details...)
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

	uploadCondition := marketplacev1alpha1.ReportConditionStorageStatusUnknown
	uploadStatuses := marketplacev1alpha1.UploadDetailConditions{}

	if r.Config.Upload {
		func() {
			logger.Info("starting file upload", "file name", fileName)

			uploader := r.Uploader

			status := &marketplacev1alpha1.UploadDetails{
				Target: uploader.Name(),
			}

			file, err := os.Open(fileName)

			if err != nil {
				logger.Error(err, "failed to upload")
				status.Status = "failure"
				status.Error = err.Error()
				uploadCondition = marketplacev1alpha1.ReportConditionStorageStatusErrored
				uploadCondition.Message = err.Error()
				uploadStatuses = append(uploadStatuses, status)
				return
			}

			reportMetadata := &dataservice.MeterReportMetadata{
				ReportName:      r.ReportName.Name,
				ReportNamespace: r.ReportName.Namespace,
			}
			ctx = context.WithValue(ctx, "metadata", reportMetadata)
			id, err := uploader.UploadFile(ctx, fileName, file)

			status.ID = id

			if err != nil {
				logger.Error(err, "failed to upload")
				status.Status = "failure"
				status.Error = err.Error()
				uploadCondition = marketplacev1alpha1.ReportConditionStorageStatusErrored
				uploadCondition.Message = err.Error()
			} else {
				logger.Info("uploaded metrics", "metricsLength", len(metrics), "target", uploader.Name())
				status.Status = "success"
				uploadCondition = marketplacev1alpha1.ReportConditionStorageStatusFinished
			}

			uploadStatuses = append(uploadStatuses, status)
		}()
	}

	status := marketplacev1alpha1.MeterReportStatus{}
	for _, err := range errorList {
		status.Errors = append(status.Errors,
			(marketplacev1alpha1.ErrorDetails{}).FromError(err),
		)
	}

	for _, err := range warningList {
		status.Warnings = append(status.Warnings,
			(marketplacev1alpha1.ErrorDetails{}).FromError(err),
		)
	}

	if !r.Config.Local {
		err = updateMeterReportStatus(ctx, r.K8SClient, r.ReportName.Name, r.ReportName.Namespace,
			func(status marketplacev1alpha1.MeterReportStatus) marketplacev1alpha1.MeterReportStatus {
				status.UploadStatus.Append(uploadStatuses)
				status.MetricUploadCount = ptr.Int(len(metrics))
				status.Errors = make([]marketplacev1alpha1.ErrorDetails, 0, len(errorList))
				status.Warnings = make([]marketplacev1alpha1.ErrorDetails, 0, len(warningList))
				status.Conditions.SetCondition(uploadCondition)

				dataServiceStatus := uploadStatuses.Get(uploaders.UploaderTargetDataService.Name())
				if dataServiceStatus != nil {
					status.DataServiceStatus = dataServiceStatus
				}

				return status
			},
		)
		if err != nil {
			log.Error(err, "failed to update report status")
		}
	}

	if len(errorList) != 0 {
		return errors.Combine(errorList...)
	}

	return nil
}

func providePrometheusSetup(
	config *Config,
	report *marketplacev1alpha1.MeterReport,
	promService *corev1.Service,
	promPort *corev1.ServicePort,
) *PrometheusAPISetup {
	return &PrometheusAPISetup{
		Report:        report,
		PromService:   promService,
		PromPort:      promPort,
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
	cc ClientCommandRunner,
	cfg *Config,
) (service *corev1.Service, returnErr error) {
	service = &corev1.Service{}
	if cfg.Local {
		return nil, nil
	}

	if cfg.PrometheusService == "" || cfg.PrometheusNamespace == "" {
		returnErr = fmt.Errorf("no prometheus configured")
		return
	}

	name := types.NamespacedName{
		Name:      cfg.PrometheusService,
		Namespace: cfg.PrometheusNamespace,
	}

	if result, _ := cc.Do(ctx, GetAction(name, service)); !result.Is(Continue) {
		returnErr = errors.Wrap(result, "failed to get report")
		return
	}

	logger.Info("retrieved prometheus service")
	return
}

func getPrometheusPort(
	cfg *Config,
	service *corev1.Service,
) (*corev1.ServicePort, error) {
	if cfg.Local {
		return nil, nil
	}

	var port *corev1.ServicePort

	for i, portB := range service.Spec.Ports {
		if portB.Name == cfg.PrometheusPort {
			port = &service.Spec.Ports[i]
		}
	}

	if port == nil {
		return nil, errors.New("cannot find port: " + cfg.PrometheusPort)
	}

	return port, nil
}

type MeterDefinitionReferences = []marketplacev1beta1.MeterDefinitionReference

func getMeterDefinitionReferences(
	ctx context.Context,
	report *marketplacev1alpha1.MeterReport,
	client client.Client,
) (defs MeterDefinitionReferences, err error) {
	for _, ref := range report.Spec.MeterDefinitionReferences {
		if ref.Spec == nil {
			mdef := marketplacev1beta1.MeterDefinition{}

			err = client.Get(ctx, types.NamespacedName{
				Name: ref.Name, Namespace: ref.Namespace,
			}, &mdef)

			if err != nil {
				return
			}

			ref.UID = mdef.UID
			ref.Spec = &mdef.Spec
		}

		defs = append(defs, ref)
	}

	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if reflect.DeepEqual(report.Spec.MeterDefinitionReferences, defs) {
			return nil
		}

		report.Spec.MeterDefinitionReferences = defs

		return client.Update(ctx, report)
	})

	return
}

// Stop() and Sleep to allow queue to write out events.
// Calling Broadcaster Shutdown() otherwise is a risk of panic and lost event
// Until we are using a newer k8s api-machinery version
// https://github.com/kubernetes/kubernetes/issues/94906
func provideReporterEventBroadcaster(kubeclientset kubernetes.Interface) (record.EventBroadcaster, func()) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	shutdownInterface := eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	return eventBroadcaster, func() {
		shutdownInterface.Stop()
		logger.Info("Wait for event broadcaster to stop...")
		time.Sleep(10 * time.Second)
	}
}

func provideReporterEventRecorder(eventBroadcaster record.EventBroadcaster, schemeIn *runtime.Scheme) record.EventRecorder {
	recorder := eventBroadcaster.NewRecorder(schemeIn, corev1.EventSource{Component: "reporter"})
	return recorder
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
	utilruntime.Must(batchv1.AddToScheme(scheme))
	return scheme
}
