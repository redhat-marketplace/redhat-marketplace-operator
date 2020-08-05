package report

import (
	"context"
	"os"
	"time"

	"emperror.dev/errors"
	"github.com/gotidy/ptr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/reporter"
	"github.com/spf13/cobra"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("reporter_report_cmd")

var name, namespace, cafile, tokenFile string
var local bool
var retry int

var ReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Run the report",
	Long:  `Runs the report. Takes it name and namespace as args`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Info("running the report command")

		if name == "" || namespace == "" {
			log.Error(errors.New("name or namespace not provided"), "namespace or name not provided")
			os.Exit(1)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		tmpDir := os.TempDir()

		cfg := &reporter.Config{
			OutputDirectory: tmpDir,
			Retry:           ptr.Int(retry),
			CaFile:          cafile,
			TokenFile:       tokenFile,
			Local:           local,
		}
		cfg.SetDefaults()

		task, err := reporter.NewTask(
			ctx,
			reporter.ReportName{Namespace: namespace, Name: name},
			cfg,
		)

		if err != nil {
			log.Error(err, "couldn't initialize task")
			os.Exit(1)
		}

		err = task.Run()
		if err != nil {
			log.Error(err, "error running task")
			os.Exit(1)
		}

		os.Exit(0)
	},
}

func init() {
	ReportCmd.Flags().StringVar(&name, "name", "", "name of the report")
	ReportCmd.Flags().StringVar(&namespace, "namespace", "", "namespace of the report")
	ReportCmd.Flags().StringVar(&cafile, "cafile", "", "cafile for prometheus")
	ReportCmd.Flags().StringVar(&tokenFile, "tokenfile", "", "token file for prometheus")
	ReportCmd.Flags().BoolVar(&local, "local", false, "run locally")
	ReportCmd.Flags().IntVar(&retry, "retry", 3, "number of retries")
}
