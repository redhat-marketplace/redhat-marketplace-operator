package reporter

import (
	"context"

	"emperror.dev/errors"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/managers"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Task struct {
	ReportName ReportName
	Cache      cache.Cache
	K8SClient  client.Client
	Ctx        context.Context
	Config     *Config
	K8SScheme  *runtime.Scheme
}

func (r *Task) Run() error {
	logger.Info("task run start")
	stopCh := make(chan struct{})
	defer close(stopCh)

	go func() {
		err := r.Cache.Start(stopCh)
		logger.Error(err, "")
	}()

	logger.Info("creating reporter")
	reporter, err := NewReporter(r)

	if err != nil {
		return err
	}

	logger.Info("starting collection")
	metrics, err := reporter.CollectMetrics(r.Ctx)

	if err != nil {
		logger.Error(err, "error collecting metrics")
		return err
	}

	logger.Info("metrics", "metrics", metrics)
	return nil
}

func getClientOptions() managers.ClientOptions {
	return managers.ClientOptions{
		Namespace:    "",
		DryRunClient: false,
	}
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
	report *marketplacev1alpha1.MeterReport,
) []*marketplacev1alpha1.MeterDefinitionSpec {
	return report.Spec.MeterDefinitions
}
