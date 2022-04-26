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
	"time"

	"github.com/redhat-marketplace/redhat-marketplace-operator/authchecker/v2/pkg/authchecker"
	"github.com/spf13/cobra"
	authv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	rootCmd = &cobra.Command{
		Use:   "authvalid",
		Short: "Checks if kube auth is still valid.",
		Run:   run,
	}

	podname, namespace string
	retry              int64

	pprofEnabled bool

	log = logf.Log.WithName("authvalid_cmd")

	scheme   = runtime.NewScheme()
	setupLog = log.WithName("setup")

	httpAddr string
)

func init() {
	utilruntime.Must(authv1.AddToScheme(scheme))

	logf.SetLogger(zap.New(zap.UseDevMode(true)))
	rootCmd.Flags().StringVar(&httpAddr, "http-addr", ":28088", "The address the probe endpoint binds to.")
	rootCmd.Flags().Int64Var(&retry, "retry", 30, "retry time in seconds")
	rootCmd.Flags().BoolVar(&pprofEnabled, "pprof", os.Getenv("PPROF_DEBUG") == "true", "enable pprof")
	rootCmd.Flags().MarkHidden("pprof")
}

func run(cmd *cobra.Command, args []string) {
	restConfig := ctrl.GetConfigOrDie()

	client, err := client.New(restConfig, client.Options{
		Scheme: scheme,
	})

	if err != nil {
		setupLog.Error(err, "error starting auth checker")
		er(err)
	}

	ac := &authchecker.AuthCheckChecker{
		RetryTime: time.Duration(retry) * time.Second,
		Logger:    log,
		Client:    client,
		FilePath:  "/var/run/secrets/kubernetes.io/serviceaccount/token",
	}

	ctx := ctrl.SetupSignalHandler()

	if err := ac.Start(ctx); err != nil {
		setupLog.Error(err, "failed to add runnable")
		os.Exit(1)
	}

	r := http.NewServeMux()

	healthzHandler := &healthz.Handler{Checks: map[string]healthz.Checker{}}
	healthzHandler.Checks["health"] = ac.Check
	r.Handle("/healthz", http.StripPrefix("/healthz", healthzHandler))

	readyHandler := &healthz.Handler{Checks: map[string]healthz.Checker{}}
	readyHandler.Checks["ready"] = ac.Check
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

func er(msg interface{}) {
	fmt.Println("Error:", msg)
	os.Exit(1)
}

func main() {
	err := rootCmd.Execute()

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	os.Exit(0)
}
