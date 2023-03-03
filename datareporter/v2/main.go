/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"os"

	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/fsnotify/fsnotify"
	datareporterv1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/api/v1alpha1"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/api/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/controllers"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/events"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/server"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	//+kubebuilder:scaffold:imports

	"go.uber.org/zap/zapcore"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(datareporterv1alpha1.AddToScheme(scheme))
	utilruntime.Must(marketplacev1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string

	// dataReporter flags
	var namespace string
	var dataServiceCertFile string
	var dataServiceTokenFile string
	var componentConfigVar string

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	// dataReporter flags
	flag.StringVar(&dataServiceTokenFile, "dataServiceTokenFile", "/etc/data-service-sa/data-service-token", "token file for the data service")
	flag.StringVar(&dataServiceCertFile, "dataServiceCertFile", "/etc/configmaps/serving-cert-ca-bundle/service-ca.crt", "cert file for the data service")
	flag.StringVar(&namespace, "namespace", os.Getenv("POD_NAMESPACE"), "namespace where the operator is deployed")

	// componentconfig path
	flag.StringVar(&componentConfigVar, "config", "",
		"The controller will load its initial configuration from this file. "+
			"Omit this flag to use the default configuration values. "+
			"Command-line flags override configuration from this file.")

	opts := zap.Options{
		Development: true,
		EncoderConfigOptions: []zap.EncoderConfigOption{
			func(ec *zapcore.EncoderConfig) {
				ec.EncodeTime = zapcore.ISO8601TimeEncoder
			},
		},
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config := &events.Config{
		OutputDirectory:      os.TempDir(),
		DataServiceTokenFile: dataServiceTokenFile,
		DataServiceCertFile:  dataServiceCertFile,
		Namespace:            namespace,
	}

	setupLog.Info("componentConfigVar", "file", componentConfigVar)
	viper.SetConfigFile(componentConfigVar)

	if err := viper.ReadInConfig(); err != nil {
		setupLog.Error(err, "Error reading config file")
	}

	componentConfig := datareporterv1alpha1.ComponentConfig{}
	err := viper.Unmarshal(&componentConfig)
	if err != nil {
		setupLog.Error(err, "error unmarshaling")
	}

	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		// fmt.Println("Config file changed:", e.Name)
		setupLog.Info("config file changed", "file", e.Name)
		err = viper.Unmarshal(&componentConfig)
		if err != nil {
			setupLog.Error(err, "error unmarshaling")
		}
		utils.PrettyPrint(componentConfig)
	})

	utils.PrettyPrintWithLog(componentConfig, "project config:")

	eventEngine := events.NewEventEngine(ctx, ctrl.Log, config)
	err = eventEngine.Start(ctx)
	if err != nil {
		setupLog.Error(err, "unable to start engine")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "datareporter.marketplace.redhat.com",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.DataReporterConfigReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Config: config,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DataReporterConfig")
		os.Exit(1)
	}

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// go func() {
	// 	eventjson := json.RawMessage(`{"event":"one"}`)
	// 	eventone := events.Event{Key: "one", RawMessage: eventjson}
	// 	for i := 0; i < 3; i++ {
	// 		setupLog.Info("sending event", "event", eventone)
	// 		eventEngine.EventChan <- eventone
	// 		time.Sleep(1 * time.Second)
	// 	}
	// }()

	h := server.NewDataReporterHandler(eventEngine, config)

	if err := mgr.AddMetricsExtraHandler("/", h); err != nil {
		setupLog.Error(err, "unable to set up pprof")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
