// Copyright 2021 IBM Corp.
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
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	server "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/internal/server"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/dqlite"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/scheduler"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

var log logr.Logger

func main() {
	var (
		api            string
		gatewayApi     string
		db             string
		join           []string
		dir            string
		verbose        bool
		cleanAfter     string
		config         string
		cronExpression string
		caCert         string
		tlsCert        string
		tlsKey         string
	)

	cmd := &cobra.Command{
		Use:   "grpc-api",
		Short: "Command to start up grpc server and database",
		Long:  `Command to start up grpc server and establish a database connection`,
		RunE: func(cmd *cobra.Command, args []string) error {

			// reads config file using viper
			if len(strings.TrimSpace(config)) != 0 {
				viper.SetConfigFile(config)
				if err := viper.ReadInConfig(); err != nil {
					log.Error(err, "Error reading config file")
				}
			}

			j := viper.GetStringSlice("join")
			cfg := &dqlite.DatabaseConfig{
				Name:    "airgap",
				Dir:     viper.GetString("dir"),
				Url:     viper.GetString("db"),
				Join:    &j,
				Verbose: viper.GetBool("verbose"),
				Log:     log,
				CACert:  caCert,
				TLSCert: tlsCert,
				TLSKey:  tlsKey,
			}

			cleanAfter := viper.GetDuration("cleanAfter")
			fs, err := cfg.InitDB(cleanAfter)
			if err != nil {
				return err
			}

			defer cfg.Close()

			sfg := &scheduler.SchedulerConfig{
				Log:            log,
				Fs:             fs,
				IsLeader:       cfg.IsLeader,
				CleanAfter:     viper.GetString("cleanAfter"),
				CronExpression: viper.GetString("cronExpression"),
			}

			// Attempt migration and start scheduler
			err = cfg.TryMigrate()
			if err != nil {
				return err
			}

			sfg.StartScheduler()

			ctx, cancel := context.WithCancel(context.Background())

			bs := &server.Server{Log: log, FileStore: fs, APIEndpoint: api, GatewayEndpoint: gatewayApi}

			stopCh := (&shutdownHandler{log: log}).SetupSignalHandler()

			var group errgroup.Group

			group.Go(func() error {
				return bs.Start(ctx)
			})

			// handle signals
			group.Go(func() error {
				<-stopCh
				cancel()
				return nil
			})

			return group.Wait()
		},
	}

	flags := cmd.Flags()

	flags.StringVarP(&api, "api", "a", "127.0.0.1:8003", "address used to expose the grpc API")
	flags.StringVarP(&gatewayApi, "gw", "g", "127.0.0.1:8007", "address used to expose the grpc gateway API")

	flags.StringVarP(&db, "db", "d", "", "address used for internal database replication")

	flags.StringSliceVarP(&join, "join", "j", nil, "database addresses of existing nodes")
	flags.StringVarP(&dir, "dir", "D", "/tmp/dqlite", "data directory")
	flags.BoolVarP(&verbose, "verbose", "v", false, "verbose logging")
	flags.StringVarP(&caCert, "ca-cert", "", "", "CA certificate")
	flags.StringVarP(&tlsCert, "tls-cert", "", "", "x509 certificate")
	flags.StringVarP(&tlsKey, "tls-key", "", "", "x509 private key")
	flags.StringVar(&config, "config", "", "path to config file")

	flags.StringVar(&cleanAfter, "cleanAfter", "-1440h", "clean files older than x seconds/minutes/hours, default 1440i.e. 60 days")
	flags.StringVar(&cronExpression, "cronExpression", "0 0 * * *", "cron expression for scheduler, default cron will run every day 12:00 AM")

	cmd.MarkFlagRequired("db")

	viper.BindPFlags(flags)

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var onlyOneSignalHandler = make(chan struct{})
var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

type shutdownHandler struct {
	log logr.Logger
}

func (s *shutdownHandler) SetupSignalHandler() (stopCh <-chan struct{}) {
	close(onlyOneSignalHandler) // panics when called twice

	stop := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, shutdownSignals...)
	go func() {
		<-c
		s.log.Info("shutdown signal received")
		close(stop)
		<-c
		s.log.Info("second shutdown signal received, killing")
		os.Exit(1) // second signal. Exit directly.
	}()

	return stop
}

func init() {
	zapLog, err := zap.NewDevelopment()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize zapr, due to error: %v", err))
	}
	log = zapr.NewLogger(zapLog)
}
