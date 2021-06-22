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
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/adminserver"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/fileretreiver"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/filesender"
	server "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/cmd/server/start"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/dqlite"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/scheduler"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
)

var log logr.Logger

func main() {
	var (
		api            string
		db             string
		join           []string
		dir            string
		verbose        bool
		cleanAfter     string
		purgeAfter     string
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

			fs, err := cfg.InitDB()
			if err != nil {
				return err
			}

			sfg := &scheduler.SchedulerConfig{
				Log:            log,
				Fs:             fs,
				DBConfig:       *cfg,
				CleanAfter:     viper.GetString("cleanAfter"),
				PurgeAfter:     viper.GetString("purgeAfter"),
				CronExpression: viper.GetString("cronExpression"),
			}

			// Attempt migration and start scheduler
			err = cfg.TryMigrate()
			if err != nil {
				return err
			}
			sfg.StartScheduler()

			lis, err := net.Listen("tcp", api)
			if err != nil {
				return err
			}

			s := grpc.NewServer()
			bs := server.BaseServer{Log: log, FileStore: fs}

			//Register servers
			filesender.RegisterFileSenderServer(s, &server.FileSenderServer{B: bs})
			fileretreiver.RegisterFileRetreiverServer(s, &server.FileRetreiverServer{B: bs})
			adminserver.RegisterAdminServerServer(s, &server.AdminServerServer{B: bs})

			go func() {
				if err := s.Serve(lis); err != nil {
					panic(err)
				}
			}()

			ch := make(chan os.Signal)
			signal.Notify(ch, unix.SIGINT)
			signal.Notify(ch, unix.SIGQUIT)
			signal.Notify(ch, unix.SIGTERM)

			<-ch

			lis.Close()
			cfg.Close()

			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&api, "api", "a", "", "address used to expose the grpc API")
	flags.StringVarP(&db, "db", "d", "", "address used for internal database replication")
	flags.StringSliceVarP(&join, "join", "j", nil, "database addresses of existing nodes")
	flags.StringVarP(&dir, "dir", "D", "/tmp/dqlite", "data directory")
	flags.BoolVarP(&verbose, "verbose", "v", false, "verbose logging")
	flags.StringVarP(&caCert, "ca-cert", "", "", "CA certificate")
	flags.StringVarP(&tlsCert, "tls-cert", "", "", "x509 certificate")
	flags.StringVarP(&tlsKey, "tls-key", "", "", "x509 private key")
	flags.StringVar(&config, "config", "", "path to config file")
	flags.StringVar(&cleanAfter, "cleanAfter", "-720h", "clean files older than x seconds/minutes/hours, default 720h i.e. 30 days")
	flags.StringVar(&purgeAfter, "purgeAfter", "-1440h", "purge files older than x seconds/minutes/hours, default 1440h i.e. 60 days")
	flags.StringVar(&cronExpression, "cronExpression", "0 0 * * *", "cron expression for scheduler, default cron will run every day 12:00 AM")

	cmd.MarkFlagRequired("api")
	cmd.MarkFlagRequired("db")

	viper.BindPFlags(flags)

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	zapLog, err := zap.NewDevelopment()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize zapr, due to error: %v", err))
	}
	log = zapr.NewLogger(zapLog)
}
