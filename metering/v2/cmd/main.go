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

	homedir "github.com/mitchellh/go-homedir"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	// Used for flags.
	cfgFile string

	opts = &server.Options{}

	rootCmd = &cobra.Command{
		Use:   "redhat-marketplace-metrics",
		Short: "Report Meter data for Red Hat Marketplace.",
		Run:   run,
	}

	log = logf.Log.WithName("metrics_cmd")
)

func run(cmd *cobra.Command, args []string) {
	log.Info("serving metrics")

	localServer, err := server.NewServer(opts)

	if err != nil {
		log.Error(err, "failed to get server")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	err = localServer.Serve(ctx.Done())

	if err != nil {
		log.Error(err, "error running server")
		cancel()
		os.Exit(1)
	}

	os.Exit(0)
}

func init() {
	logf.SetLogger(zap.New(zap.UseDevMode(true)))
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cobra.yaml)")
	opts.Mount(rootCmd.PersistentFlags().AddFlagSet)
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
