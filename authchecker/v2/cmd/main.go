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
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/redhat-marketplace/redhat-marketplace-operator/authchecker/v2/pkg/authchecker"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
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

	log = logf.Log.WithName("authvalid_cmd")

	scheme   = runtime.NewScheme()
	setupLog = log.WithName("setup")

	metricsAddr, probeAddr string
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	logf.SetLogger(zap.New(zap.UseDevMode(true)))
	rootCmd.Flags().StringVar(&probeAddr, "health-probe-bind-address", ":8089", "The address the probe endpoint binds to.")
	rootCmd.Flags().StringVar(&namespace, "namespace", "", "namespace to list")
	rootCmd.Flags().StringVar(&podname, "podname", os.Getenv("POD_NAME"), "podname")
	rootCmd.Flags().Int64Var(&retry, "retry", 30, "retry count")
}

func run(cmd *cobra.Command, args []string) {
	log.Info("starting up with vars",
		"podname", podname,
		"namespace", namespace,
		"retry", retry,
	)

	if namespace == "" {
		setupLog.Error(errors.New("namespace not provided"), "namespace not provided", "namespace", namespace)
		os.Exit(1)
	}

	opts := ctrl.Options{
		Scheme:                 scheme,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         false,
		Namespace:              namespace,
		MetricsBindAddress:     "0",
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), opts)
	if err != nil {
		log.Error(err, "error starting auth checker")
		er(err)
	}

	ac := &authchecker.AuthChecker{
		RetryTime: time.Duration(retry) * time.Second,
		Podname:   podname,
		Namespace: namespace,
		Logger:    log,
		Client:    mgr.GetClient(),
		Checker:   &authchecker.AuthCheckChecker{},
	}
	err = (ac).SetupWithManager(mgr)

	if err != nil {
		setupLog.Error(err, "error starting auth checker")
		er(err)
	}

	if err := mgr.AddHealthzCheck("health", ac.Checker.Check); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}

	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
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
