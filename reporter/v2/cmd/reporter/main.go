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

package main

import (
	"fmt"
	"os"

	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/cmd/reporter/reconciler"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/cmd/reporter/report"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/cmd/reporter/sign"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/cmd/reporter/verify"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap/zapcore"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	// Used for flags.
	cfgFile string

	rootCmd = &cobra.Command{
		Use:   "redhat-marketplace-reporter",
		Short: "Report Meter data for Red Hat Marketplace.",
	}
)

func init() {
	encoderConfig := func(ec *zapcore.EncoderConfig) {
		ec.EncodeTime = zapcore.ISO8601TimeEncoder
	}
	zapOpts := func(o *zap.Options) {
		o.EncoderConfigOptions = append(o.EncoderConfigOptions, encoderConfig)
	}

	logf.SetLogger(zap.New(zap.UseDevMode(true), zapOpts))
	cobra.OnInitialize(initConfig)

	rootCmd.AddCommand(report.ReportCmd)
	rootCmd.AddCommand(sign.SignCmd)
	rootCmd.AddCommand(verify.VerifyCmd)
	rootCmd.AddCommand(reconciler.ReconcileCmd)
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
		home, err := os.UserHomeDir()
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

	os.Exit(0)
}
