package main

import (
	"fmt"
	"os"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/redhat-marketplace/redhat-marketplace-operator/cmd/reporter/report"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/metric_generator"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
)

var (
	// Used for flags.
	cfgFile string

	opts = &metric_generator.Options{}

	rootCmd = &cobra.Command{
		Use:   "redhat-marketplace-metrics",
		Short: "Report Meter data for Red Hat Marketplace.",
		Run: run,
	}

	log = logger.NewLogger("metrics_cmd")
)

func run(cmd *cobra.Command, args []string) {
	log.Info("serving metrics")

	server, err := metric_generator.NewServer(opts)

	if err != nil {
		log.Error(err, "failed to get server")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server.Serve(ctx.Done())

	os.Exit(0)
}

func init() {
	logger.SetLoggerToDevelopmentZap()
	cobra.OnInitialize(initConfig)

	rootCmd.AddCommand(report.ReportCmd)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cobra.yaml)")
	opts.Mount(rootCmd.Flags().AddFlagSet)
}

func er(msg interface{}) {
	fmt.Println("Error:", msg)
	os.Exit(1)
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			er(err)
		}

		// Search config in home directory with name ".cobra" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".cobra")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func main() {
	err := rootCmd.Execute()

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
