package report

import (
	"context"
	"os"

	"emperror.dev/errors"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/reporter"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var log = logf.Log.WithName("reporter_report")

var name, namespace string

var ReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Print the version number of Hugo",
	Long:  `All software has versions. This is Hugo's`,
	Run: func(cmd *cobra.Command, args []string) {
		outputDir := os.TempDir()
		cfg := reporter.Config{
			OutputDirectory: outputDir,
		}

		ctx := context.TODO()

		report, err := initializeMarketplaceReporter(
			ctx,
			reporter.ReportName{Namespace: namespace, Name: name},
			cfg,
		)

		if err != nil {
			log.Error(err, "")
			os.Exit(1)
		}

		metrics, err := report.CollectMetrics(ctx)

		if err != nil {
			log.Error(err, "")
			os.Exit(1)
		}

		log.Info("metrics", "metrics", metrics)

		os.Exit(0)
	},
}

func init() {
	ReportCmd.Flags().StringVar(&name, "name", "", "name of the report")
	ReportCmd.Flags().StringVar(&name, "namespace", "", "namespace of the report")
}

func provideOptions(kscheme *runtime.Scheme) (*manager.Options, error) {
	watchNamespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		log.Error(err, "Failed to get watch namespace")
		return nil, err
	}

	return &manager.Options{
		Namespace:          watchNamespace,
		Scheme:             kscheme,
	}, nil
}

func getMarketplaceReport(
	ctx context.Context,
	cc ClientCommandRunner,
	reportName reporter.ReportName,
) (report *marketplacev1alpha1.MeterReport, returnErr error) {
	report = &marketplacev1alpha1.MeterReport{}

	if result, _ := cc.Do(ctx, GetAction(types.NamespacedName(reportName), report)); !result.Is(Continue) {
		returnErr = errors.Wrap(result, "failed to get report")
	}

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

	return
}

func getMeterDefinitions(
	report *marketplacev1alpha1.MeterReport,
) []*marketplacev1alpha1.MeterDefinition {
	return report.Spec.MeterDefinitions
}
