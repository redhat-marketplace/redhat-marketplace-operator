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
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	retryablehttp "github.com/hashicorp/go-retryablehttp"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/api/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/controllers"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/datafilter"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/events"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/filters"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/logger"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/server"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	k8sapiflag "k8s.io/component-base/cli/flag"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

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
	var tlsCert string
	var tlsKey string
	var tlsMinVersion string
	var tlsCipherSuites []string
	var componentConfigVar string

	pflag.StringVar(&namespace, "namespace", os.Getenv("POD_NAMESPACE"), "namespace where the operator is deployed")

	// TLS
	pflag.StringVar(&tlsCert, "tls-cert-file", "/etc/tls/private/tls.crt", "TLS certificate file path")
	pflag.StringVar(&tlsKey, "tls-private-key-file", "/etc/tls/private/tls.key", "TLS private key file path")
	pflag.StringVar(&tlsMinVersion, "tls-min-version", "VersionTLS12", "Minimum TLS version supported. Value must match version names from https://golang.org/pkg/crypto/tls/#pkg-constants.")
	pflag.StringSliceVar(&tlsCipherSuites,
		"tls-cipher-suites",
		[]string{"TLS_AES_128_GCM_SHA256",
			"TLS_AES_256_GCM_SHA384",
			"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
			"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"},
		"Comma-separated list of cipher suites for the server. Values are from tls package constants (https://golang.org/pkg/crypto/tls/#pkg-constants). If omitted, a subset will be used")

	// componentconfig path
	pflag.StringVar(&componentConfigVar, "config", "",
		"The controller will load its initial configuration from this file. "+
			"Omit this flag to use the default configuration values. "+
			"Command-line flags override configuration from this file.")

	pflag.Parse()

	opts := zap.Options{
		Development: true,
		EncoderConfigOptions: []zap.EncoderConfigOption{
			func(ec *zapcore.EncoderConfig) {
				ec.EncodeTime = zapcore.ISO8601TimeEncoder
			},
		},
	}
	opts.BindFlags(flag.CommandLine)

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	// Outbound TLS to DataService, from ComponentConfig

	tlsVersion, err := k8sapiflag.TLSVersion(cc.TLSConfig.MinVersion)
	if err != nil {
		setupLog.Error(err, "TLS version invalid")
		os.Exit(1)
	}

	cipherSuites, err := k8sapiflag.TLSCipherSuites(cc.TLSConfig.CipherSuites)
	if err != nil {
		setupLog.Error(err, "failed to convert TLS cipher suite name to ID")
		os.Exit(1)
	}

	dataServiceURL, err := url.Parse(fmt.Sprintf("%s.%s.svc:8004", utils.DATA_SERVICE_NAME, namespace))
	if err != nil {
		setupLog.Error(err, "failed to parse for dataServiceURL")
		os.Exit(1)
	}

	config := &events.Config{
		OutputDirectory:      os.TempDir(),
		DataServiceTokenFile: cc.DataServiceTokenFile,
		DataServiceCertFile:  cc.DataServiceCertFile,
		DataServiceURL:       dataServiceURL,
		Namespace:            namespace,
		AccMemoryLimit:       cc.AccMemoryLimit,
		MaxFlushTimeout:      cc.MaxFlushTimeout,
		MaxEventEntries:      cc.MaxEventEntries,
		CipherSuites:         cipherSuites,
		MinVersion:           tlsVersion,
	}

	//
	// Service TLS Configuration, from CommandLine flagset
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

	cacheOptions := cache.Options{
		ByObject: map[client.Object]cache.ByObject{
			&v1alpha1.DataReporterConfig{}: {
				Field: fields.SelectorFromSet(fields.Set{
					"metadata.namespace": os.Getenv("POD_NAMESPACE")}),
			},
			&corev1.Service{}: {
				Field: fields.SelectorFromSet(fields.Set{
					"metadata.namespace": os.Getenv("POD_NAMESPACE")}),
			},
			&corev1.ConfigMap{}: {
				Field: fields.SelectorFromSet(fields.Set{
					"metadata.namespace": os.Getenv("POD_NAMESPACE")}),
			},
			&corev1.Secret{}: {
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
	}

	// Due to controller-manager v0.16 change, the handler must now be defined before NewManager
	// Thus mgr.GetClient() is not yet available, as such Client is set to nil for now

	eventEngine := events.NewEventEngine(ctx, ctrl.Log, config, nil)

	rc := retryablehttp.NewClient()
	rc.Logger = logger.NewRetryableHTTPLogger()
	sc := rc.StandardClient() // *http.Client
	dataFilters := datafilter.NewDataFilters(ctrl.Log.WithName("datafilter"), nil, sc, eventEngine, config, &cc.ApiHandlerConfig)

	// Add the EventEngine handler
	h := server.NewDataReporterHandler(eventEngine, config, dataFilters, &cc.ApiHandlerConfig)
	extraHandlers := make(map[string]http.Handler)
	extraHandlers["/"] = h

	metricsOpts := metricsserver.Options{
		BindAddress:    cc.Metrics.BindAddress,
		SecureServing:  true,
		FilterProvider: filters.WithAuthenticationAndAuthorization,
		ExtraHandlers:  extraHandlers,
		TLSOpts: []func(*tls.Config){
			func(cfg *tls.Config) {
				cfg.GetCertificate = watcher.GetCertificate
				cfg.MinVersion = tlsMV
				cfg.CipherSuites = tlsCS
			},
		},
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsOpts,
		HealthProbeBindAddress: cc.ManagerConfig.Health.HealthProbeBindAddress,
		LeaderElection:         *cc.ManagerConfig.LeaderElection.LeaderElect,
		LeaderElectionID:       cc.ManagerConfig.LeaderElectionID,
		Cache:                  cacheOptions,
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
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	if err = (&controllers.DataReporterConfigReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		Config:      config,
		Log:         ctrl.Log.WithName("controllers").WithName("DataReporterConfigController"),
		DataFilters: dataFilters,
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

	// mgr.GetClient() is now available
	eventEngine.SetKubeClient(mgr.GetClient())
	dataFilters.SetKubeClient(mgr.GetClient())

	go func() {
		err := eventEngine.Start(ctx)
		if err != nil {
			setupLog.Error(err, "unable to start engine")
			os.Exit(1)
		}
	}()

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
