/*
Copyright 2020 IBM Co..

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
	"crypto/tls"
	"os"

	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	flag "github.com/spf13/pflag"
	"go.uber.org/zap/zapcore"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/discovery"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	k8sapiflag "k8s.io/component-base/cli/flag"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	osimagev1 "github.com/openshift/api/image/v1"
	routev1 "github.com/openshift/api/route/v1"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	osappsv1 "github.com/openshift/api/apps/v1"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	controllers "github.com/redhat-marketplace/redhat-marketplace-operator/v2/controllers/marketplace"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"

	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/prometheus"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/runnables"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

const (
	WebhookCertDir  = "/apiserver.local.config/certificates"
	WebhookCertName = "apiserver.crt"
	WebhookKeyName  = "apiserver.key"
	WebhookPort     = 9443
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(marketplacev1alpha1.AddToScheme(scheme))
	utilruntime.Must(openshiftconfigv1.AddToScheme(scheme))
	utilruntime.Must(olmv1.AddToScheme(scheme))
	utilruntime.Must(olmv1alpha1.AddToScheme(scheme))
	utilruntime.Must(monitoringv1.AddToScheme(scheme))
	utilruntime.Must(marketplacev1beta1.AddToScheme(scheme))
	utilruntime.Must(routev1.AddToScheme(scheme))
	utilruntime.Must(osappsv1.AddToScheme(scheme))
	mktypes.RegisterImageStream(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var profBindAddress string
	var tlsCert string
	var tlsKey string
	var tlsMinVersion string
	var tlsCipherSuites []string

	flag.StringVar(&metricsAddr, "metrics-addr", ":8443", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&profBindAddress, "pprof-bind-address", "0", "The address the pprof endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&tlsCert, "tls-cert-file", "/etc/tls/private/tls.crt", "TLS certificate file path")
	flag.StringVar(&tlsKey, "tls-private-key-file", "/etc/tls/private/tls.key", "TLS private key file path")
	flag.StringVar(&tlsMinVersion, "tls-min-version", "VersionTLS12", "Minimum TLS version supported. Value must match version names from https://golang.org/pkg/crypto/tls/#pkg-constants.")
	flag.StringSliceVar(&tlsCipherSuites,
		"tls-cipher-suites",
		[]string{"TLS_AES_128_GCM_SHA256",
			"TLS_AES_256_GCM_SHA384",
			"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
			"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"},
		"Comma-separated list of cipher suites for the server. Values are from tls package constants (https://golang.org/pkg/crypto/tls/#pkg-constants). If omitted, a subset will be used")

	flag.Parse()

	encoderConfig := func(ec *zapcore.EncoderConfig) {
		ec.EncodeTime = zapcore.ISO8601TimeEncoder
	}
	zapOpts := func(o *zap.Options) {
		o.EncoderConfigOptions = append(o.EncoderConfigOptions, encoderConfig)
	}

	ctrl.SetLogger(zap.New(zap.UseDevMode(true), zapOpts))

	ctx := ctrl.SetupSignalHandler()

	//
	// TLS Configuration
	//

	tlsMV, err := k8sapiflag.TLSVersion(tlsMinVersion)
	if err != nil {
		setupLog.Error(err, "tls-min-version TLS min version invalid", "flag", "tls-min-version")
		os.Exit(1)
	}

	tlsCS, err := k8sapiflag.TLSCipherSuites(tlsCipherSuites)
	if err != nil {
		setupLog.Error(err, "failed to convert TLS cipher suite name to ID", "flag", "tls-cipher-suites")
		os.Exit(1)
	}

	// Initialize a new cert watcher with cert/key pair
	watcher, err := certwatcher.New(tlsCert, tlsKey)
	if err != nil {
		setupLog.Error(err, "failed to create certwatcher")
		os.Exit(1)
	}

	// Start goroutine with certwatcher running fsnotify against supplied certdir
	go func() {
		if err := watcher.Start(ctx); err != nil {
			setupLog.Error(err, "failed to start certwatcher")
			os.Exit(1)
		}
	}()

	// Reconciler does not operate on copies.
	csvLabelSelector, err := labels.Parse("!olm.copiedFrom")
	if err != nil {
		setupLog.Error(err, "failed to create csvLabelSelector")
		os.Exit(1)
	}

	// This pod namespace
	nsScopePod := make(map[string]cache.Config)
	nsScopePod[os.Getenv("POD_NAMESPACE")] = cache.Config{}

	// openshift-marketplace CatalogSources
	nsScopeMkt := make(map[string]cache.Config)
	nsScopeMkt[utils.OPERATOR_MKTPLACE_NS] = cache.Config{}

	// Local StatefulSet and User Workload Monitoring
	nsScopeStS := make(map[string]cache.Config)
	nsScopeStS[os.Getenv("POD_NAMESPACE")] = cache.Config{}
	nsScopeStS[utils.OPENSHIFT_USER_WORKLOAD_MONITORING_NAMESPACE] = cache.Config{}

	// only cache these Object types a Namespace scope to reduce RBAC permission requirements
	cacheOptions := cache.Options{
		ByObject: map[client.Object]cache.ByObject{
			&corev1.Pod{}: {
				Namespaces: nsScopePod,
			},
			&corev1.Secret{}: {
				Namespaces: nsScopePod,
			},
			&corev1.ServiceAccount{}: {
				Namespaces: nsScopePod,
			},
			&corev1.PersistentVolumeClaim{}: {
				Namespaces: nsScopePod,
			},
			&appsv1.Deployment{}: {
				Namespaces: nsScopePod,
			},
			&appsv1.StatefulSet{}: {
				Namespaces: nsScopeStS,
			},
			&batchv1.CronJob{}: {
				Namespaces: nsScopePod,
			},
			&batchv1.Job{}: {
				Namespaces: nsScopePod,
			},
			&marketplacev1alpha1.MarketplaceConfig{}: {
				Namespaces: nsScopePod,
			},
			&marketplacev1alpha1.MeterBase{}: {
				Namespaces: nsScopePod,
			},
			&marketplacev1alpha1.MeterReport{}: {
				Namespaces: nsScopePod,
			},
			&marketplacev1alpha1.RazeeDeployment{}: {
				Namespaces: nsScopePod,
			},
			&routev1.Route{}: {
				Namespaces: nsScopePod,
			},
			&osimagev1.ImageStream{}: {
				Namespaces: nsScopePod,
			},
			&olmv1alpha1.CatalogSource{}: {
				Namespaces: nsScopeMkt,
			},
			&monitoringv1.Prometheus{}: {
				Namespaces: nsScopePod,
			},
			&monitoringv1.ServiceMonitor{}: {
				Namespaces: nsScopePod,
			},
			// ClusterServiceVersion Spec is a large cache memory consumer.
			// Reconciler only operates on its metadata, and does not operate on copies.
			&olmv1alpha1.ClusterServiceVersion{}: {
				Label: csvLabelSelector,
				Transform: func(obj interface{}) (interface{}, error) {
					csv := obj.(*olmv1alpha1.ClusterServiceVersion)
					csv.Spec = olmv1alpha1.ClusterServiceVersionSpec{}
					return csv, nil
				},
			},
		},
	}

	metricsOpts := metricsserver.Options{
		BindAddress:    metricsAddr,
		SecureServing:  true,
		FilterProvider: filters.WithAuthenticationAndAuthorization,
		TLSOpts: []func(*tls.Config){
			func(cfg *tls.Config) {
				cfg.GetCertificate = watcher.GetCertificate
				cfg.MinVersion = tlsMV
				cfg.CipherSuites = tlsCS
			},
		},
	}

	opts := ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsOpts,
		HealthProbeBindAddress: probeAddr,
		PprofBindAddress:       profBindAddress,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "metering.marketplace.redhat.com",
		Cache:                  cacheOptions,
	}

	// Bug prevents limiting the namespaces
	// watchNamespaces := os.Getenv("WATCH_NAMESPACE")

	// if watchNamespaces != "" {
	// 	watchNamespacesSlice := strings.Split(watchNamespaces, ",")
	// 	watchNamespacesSlice = append(watchNamespacesSlice, "openshift-monitoring")
	// 	opts.NewCache = cache.MultiNamespacedCacheBuilder(watchNamespacesSlice)
	// }

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), opts)
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(mgr.GetConfig())
	if err != nil {
		setupLog.Error(err, "unable to create client", "client", "discoveryClient")
		os.Exit(1)
	}

	mapper, err := apiutil.NewDynamicRESTMapper(mgr.GetConfig(), mgr.GetHTTPClient())
	if err != nil {
		setupLog.Error(err, "unable to create rest mapper")
		os.Exit(1)
	}

	// uncached client
	simpleClient, err := client.New(mgr.GetConfig(), client.Options{Scheme: scheme, Mapper: mapper})
	if err != nil {
		setupLog.Error(err, "unable to create client", "client", "simpleClient")
		os.Exit(1)
	}

	opCfg, err := config.ProvideInfrastructureAwareConfig(simpleClient, discoveryClient)
	if err != nil {
		setupLog.Error(err, "unable to create config", "config", "operatorConfig")
		os.Exit(1)
	}

	factory := manifests.NewFactory(opCfg, mgr.GetScheme())

	prometheusAPIBuilder := &prometheus.PrometheusAPIBuilder{
		Cfg:    opCfg,
		Client: mgr.GetClient(),
	}

	if err = (&controllers.ClusterRegistrationReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("ClusterRegistration"),
		Scheme: mgr.GetScheme(),
		Cfg:    opCfg,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterRegistration")
		os.Exit(1)
	}

	if err = (&controllers.ClusterServiceVersionReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("ClusterServiceVersion"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterServiceVersion")
		os.Exit(1)
	}

	if err = (&controllers.DataServiceReconciler{
		Client:  mgr.GetClient(),
		Log:     ctrl.Log.WithName("controllers").WithName("DataService"),
		Scheme:  mgr.GetScheme(),
		Cfg:     opCfg,
		Factory: factory,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DataService")
		os.Exit(1)
	}

	if err = (&controllers.DeploymentReconciler{
		Client:  mgr.GetClient(),
		Log:     ctrl.Log.WithName("controllers").WithName("DeploymentReconciler"),
		Scheme:  mgr.GetScheme(),
		Factory: factory,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DeploymentReconciler")
		os.Exit(1)
	}

	if err = (&controllers.MarketplaceConfigReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("MarketplaceConfig"),
		Scheme: mgr.GetScheme(),
		Cfg:    opCfg,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MarketplaceConfig")
		os.Exit(1)
	}

	if err = (&controllers.MeterBaseReconciler{
		Client:               mgr.GetClient(),
		Log:                  ctrl.Log.WithName("controllers").WithName("MeterBase"),
		Scheme:               mgr.GetScheme(),
		Cfg:                  opCfg,
		Factory:              factory,
		Recorder:             mgr.GetEventRecorderFor("meterbase-controller"),
		PrometheusAPIBuilder: prometheusAPIBuilder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MeterBase")
		os.Exit(1)
	}

	if err = (&controllers.MeterDefinitionReconciler{
		Client:               mgr.GetClient(),
		Log:                  ctrl.Log.WithName("controllers").WithName("MeterDefinition"),
		Scheme:               mgr.GetScheme(),
		Cfg:                  opCfg,
		PrometheusAPIBuilder: prometheusAPIBuilder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MeterDefinition")
		os.Exit(1)
	}

	if err = (&controllers.MeterReportReconciler{
		Client:  mgr.GetClient(),
		Log:     ctrl.Log.WithName("controllers").WithName("MeterReport"),
		Scheme:  mgr.GetScheme(),
		Cfg:     opCfg,
		Factory: factory,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MeterReport")
		os.Exit(1)
	}

	if err = (&controllers.RazeeDeploymentReconciler{
		Client:  mgr.GetClient(),
		Log:     ctrl.Log.WithName("controllers").WithName("RazeeDeployment"),
		Scheme:  mgr.GetScheme(),
		Cfg:     opCfg,
		Factory: factory,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RazeeDeployment")
		os.Exit(1)
	}

	doneChan := make(chan struct{})
	reportCreatorReconciler := &controllers.MeterReportCreatorReconciler{
		Log:    ctrl.Log.WithName("controllers").WithName("MeterReportCreator"),
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Cfg:    opCfg,
	}

	if err := reportCreatorReconciler.SetupWithManager(mgr, doneChan); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MeterReportCreator")
		os.Exit(1)
	}

	if err = (&runnables.PodMonitor{
		Logger: ctrl.Log.WithName("controllers").WithName("PodMonitor"),
		Client: mgr.GetClient(),
		Cfg:    opCfg,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PodMonitor")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}

	if err := mgr.AddReadyzCheck("check", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
	close(doneChan)
}
