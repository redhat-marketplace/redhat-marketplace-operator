package report

import (
	"os"

	"github.com/google/wire"
	"github.com/spf13/cobra"
	"github.ibm.com/symposium/redhat-marketplace-operator/pkg/managers"
	"github.ibm.com/symposium/redhat-marketplace-operator/pkg/reporter"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("reporter_report")

var name, namespace string

var ReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Print the version number of Hugo",
	Long:  `All software has versions. This is Hugo's`,
	Run: func(cmd *cobra.Command, args []string) {
		namespacedname := types.NamespacedName{Namespace: namespace, Name: name}
		report, err := initializeMarketplaceReporter(reporter.ReporterName(namespacedname))

		if err != nil {
			log.Error(err, "")
			os.Exit(1)
		}

		err = report.Run()

		if err != nil {
			log.Error(err, "")
			os.Exit(1)
		}

		os.Exit(0)
	},
}

func init() {
	ReportCmd.Flags().StringVar(&name, "name", "", "name of the report")
	ReportCmd.Flags().StringVar(&name, "namespace", "", "namespace of the report")
}

var MarketplaceReporterSet = wire.NewSet(
	managers.SchemeDefinitions,
	reporter.NewMarketplaceReporter,
	reporter.NewMarketplaceReporterConfig,
	provideMarketplaceReporterSchemes,
)

func provideMarketplaceReporterSchemes(
	monitoringScheme *managers.MonitoringSchemeDefinition,
) []*managers.SchemeDefinition {
	return []*managers.SchemeDefinition{
		(*managers.SchemeDefinition)(monitoringScheme),
	}
}
