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
	"net"
	"os"
	"os/signal"

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
		api        string
		db         string
		join       []string
		dir        string
		verbose    bool
		cleanAfter int
		purgeAfter int
		config     string
	)

	cmd := &cobra.Command{
		Use:   "grpc-api",
		Short: "Command to start up grpc server and database",
		Long:  `Command to start up grpc server and establish a database connection`,
		RunE: func(cmd *cobra.Command, args []string) error {

			// reads config file using viper
			viper.SetConfigFile(config)
			if err := viper.ReadInConfig(); err != nil {
				log.Error(err, "Error reading config file")
			}

			j := viper.GetStringSlice("join")
			cfg := &dqlite.DatabaseConfig{
				Name:    "airgap",
				Dir:     viper.GetString("dir"),
				Url:     viper.GetString("db"),
				Join:    &j,
				Verbose: viper.GetBool("verbose"),
				Log:     log,
			}

			fs, err := cfg.InitDB()
			if err != nil {
				return err
			}

			sfg := &scheduler.SchedulerConfig{
				Log:        log,
				Fs:         fs,
				CleanAfter: viper.GetInt("cleanAfter"),
				PurgeAfter: viper.GetInt("purgeAfter"),
			}

			// Attempt migration and start scheduler for leader node
			isLeader, _ := cfg.IsLeader()
			if isLeader {
				cfg.TryMigrate(context.Background())
				sfg.StartScheduler()
			}

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
	flags.StringVar(&config, "config", "config.yaml", "path to config file")
	flags.IntVar(&cleanAfter, "cleanAfter", 720, "clean files older than x da	ys")
	flags.IntVar(&purgeAfter, "purgeAfter", 1440, "purge files older than x days")

	cmd.MarkFlagRequired("api")
	cmd.MarkFlagRequired("db")
	cmd.MarkFlagRequired("config-path")

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
