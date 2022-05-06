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
	"net/http"
	"net/http/pprof"
	"os"
	"runtime/debug"
	"time"

	"github.com/redhat-marketplace/redhat-marketplace-operator/authchecker/v2/pkg/authchecker"
	"github.com/spf13/cobra"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

var (
	rootCmd = &cobra.Command{
		Use:   "authvalid",
		Short: "Checks if kube auth is still valid.",
		Run:   run,
	}

	retry        int64
	pprofEnabled bool

	log = logf.Log.WithName("authvalid_cmd")

	setupLog = log.WithName("setup")

	httpAddr string
)

func init() {
	encoderConfig := func(ec *zapcore.EncoderConfig) {
		ec.EncodeTime = zapcore.ISO8601TimeEncoder
	}
	zapOpts := func(o *zap.Options) {
		o.EncoderConfigOptions = append(o.EncoderConfigOptions, encoderConfig)
	}

	logf.SetLogger(zap.New(zap.UseDevMode(true), zapOpts))

	rootCmd.Flags().StringVar(&httpAddr, "http-addr", ":28088", "The address the probe endpoint binds to.")
	rootCmd.Flags().Int64Var(&retry, "retry", 30, "retry time in seconds")
	rootCmd.Flags().BoolVar(&pprofEnabled, "pprof", os.Getenv("PPROF_DEBUG") == "true", "enable pprof")
	rootCmd.Flags().MarkHidden("pprof")
}

func run(cmd *cobra.Command, args []string) {
	debug.SetGCPercent(50)

	restConfig := config.GetConfigOrDie()

	dynamicConfig := dynamic.NewForConfigOrDie(restConfig)

	dynamicResource := dynamicConfig.Resource(schema.GroupVersionResource{
		Group:    "authentication.k8s.io",
		Version:  "v1",
		Resource: "tokenreviews",
	})

	ac := &authchecker.AuthChecker{
		RetryTime: time.Duration(retry) * time.Second,
		Logger:    log,
		Client:    dynamicResource,
		FilePath:  "/var/run/secrets/kubernetes.io/serviceaccount/token",
	}

	ctx := signals.SetupSignalHandler()

	if err := ac.Start(ctx); err != nil {
		setupLog.Error(err, "failed to add runnable")
		os.Exit(1)
	}

	r := http.NewServeMux()

	healthzHandler := &healthz.Handler{Checks: map[string]healthz.Checker{}}
	healthzHandler.Checks["health"] = ac.Check
	r.Handle("/healthz", http.StripPrefix("/healthz", healthzHandler))

	readyHandler := &healthz.Handler{Checks: map[string]healthz.Checker{}}
	readyHandler.Checks["ready"] = healthz.Ping
	r.Handle("/readyz", http.StripPrefix("/readyz", readyHandler))

	if pprofEnabled {
		r.HandleFunc("/debug/pprof/", pprof.Index)
		r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		r.HandleFunc("/debug/pprof/profile", pprof.Profile)
		r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		r.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}

	setupLog.Info("starting server on port", "port", httpAddr)
	if err := http.ListenAndServe(httpAddr, r); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
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
