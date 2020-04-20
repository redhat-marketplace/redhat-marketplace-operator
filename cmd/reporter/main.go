package main

import (
	"fmt"
	"os"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.ibm.com/symposium/redhat-marketplace-operator/cmd/reporter/report"
)

var (
	// Used for flags.
	cfgFile     string

	rootCmd = &cobra.Command{
		Use:   "redhat-marketplace-reporter",
		Short: "Report Meter data for Red Hat Marketplace.",
	}
)

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.AddCommand(report.ReportCmd)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cobra.yaml)")
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
	rootCmd.Execute()
}
