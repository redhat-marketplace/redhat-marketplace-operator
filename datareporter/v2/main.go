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
	"runtime/debug"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	routev1 "github.com/openshift/api/route/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/api/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/controllers"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/events"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/server"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
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
	utilruntime.Must(routev1.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(marketplacev1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {

	var namespace string
	var componentConfigVar string

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

	quantity, err := resource.ParseQuantity(os.Getenv("LIMITSMEMORY"))
	if err == nil {
		setupLog.Info("setting memory limit from container resources.limits.memory", "downwardAPIEnv", "LIMITSMEMORY", "GOMEMLIMIT", quantity.String())
		debug.SetMemoryLimit(quantity.Value())
	}

	setupLog.Info("componentConfigVar", "file", componentConfigVar)

	content, err := os.ReadFile(componentConfigVar)
	if err != nil {
		setupLog.Error(err, "os.ReadFile")
	}

	codecs := serializer.NewCodecFactory(scheme)

	cc := v1alpha1.NewComponentConfig()
	if err = runtime.DecodeInto(codecs.UniversalDecoder(), content, cc); err != nil {
		setupLog.Error(err, "could not decode file into runtime.Object")
	}

	if err != nil {
		setupLog.Error(err, "unable to load the config file")
		os.Exit(1)
	}

	config := &events.Config{
		OutputDirectory:      os.TempDir(),
		DataServiceTokenFile: cc.DataServiceTokenFile,
		DataServiceCertFile:  cc.DataServiceCertFile,
		Namespace:            namespace,
		AccMemoryLimit:       cc.AccMemoryLimit,
		MaxFlushTimeout:      cc.MaxFlushTimeout,
		MaxEventEntries:      cc.MaxEventEntries,
	}

	eventEngine := events.NewEventEngine(ctx, ctrl.Log, config)
	err = eventEngine.Start(ctx)
	if err != nil {
		setupLog.Error(err, "unable to start engine")
		os.Exit(1)
	}

	newCacheFunc := cache.BuilderWithOptions(cache.Options{
		SelectorsByObject: cache.SelectorsByObject{
			&v1alpha1.DataReporterConfig{}: {
				Field: fields.SelectorFromSet(fields.Set{
					"metadata.namespace": os.Getenv("POD_NAMESPACE")}),
			},
			&corev1.Service{}: {
				Field: fields.SelectorFromSet(fields.Set{
					"metadata.namespace": os.Getenv("POD_NAMESPACE")}),
			},
			&routev1.Route{}: {
				Field: fields.SelectorFromSet(fields.Set{
					"metadata.namespace": os.Getenv("POD_NAMESPACE")}),
			},
			&marketplacev1alpha1.MarketplaceConfig{}: {
				Field: fields.SelectorFromSet(fields.Set{
					"metadata.namespace": os.Getenv("POD_NAMESPACE")}),
			},
		},
	})

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     cc.Metrics.BindAddress,
		Port:                   9443,
		HealthProbeBindAddress: cc.ManagerConfig.Health.HealthProbeBindAddress,
		LeaderElection:         *cc.ManagerConfig.LeaderElection.LeaderElect,
		LeaderElectionID:       cc.ManagerConfig.LeaderElectionID,
		NewCache:               newCacheFunc,
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
		Log:    ctrl.Log.WithName("controllers").WithName("DataReporterConfigController"),
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

	h := server.NewDataReporterHandler(eventEngine, config, cc.ApiHandlerConfig)

	if err := mgr.AddMetricsExtraHandler("/", h); err != nil {
		setupLog.Error(err, "unable to set up data reporter handler")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
