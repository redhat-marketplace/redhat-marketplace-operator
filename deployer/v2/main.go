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
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	"go.uber.org/zap/zapcore"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/discovery"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	opsrcv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	_ "net/http/pprof"

	osappsv1 "github.com/openshift/api/apps/v1"
	razeev1alpha2 "github.com/redhat-marketplace/redhat-marketplace-operator/deployer/v2/api/razee/v1alpha2"
	controllers "github.com/redhat-marketplace/redhat-marketplace-operator/deployer/v2/controllers/marketplace"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
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
	utilruntime.Must(opsrcv1.AddToScheme(scheme))
	utilruntime.Must(olmv1alpha1.AddToScheme(scheme))
	utilruntime.Must(marketplacev1beta1.AddToScheme(scheme))
	utilruntime.Must(osappsv1.AddToScheme(scheme))
	utilruntime.Must(monitoringv1.AddToScheme(scheme))
	utilruntime.Must(razeev1alpha2.AddToScheme(scheme))
	mktypes.RegisterImageStream(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var profBindAddress string
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&profBindAddress, "pprof-bind-address", "0", "The address the pprof endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	encoderConfig := func(ec *zapcore.EncoderConfig) {
		ec.EncodeTime = zapcore.ISO8601TimeEncoder
	}
	zapOpts := func(o *zap.Options) {
		o.EncoderConfigOptions = append(o.EncoderConfigOptions, encoderConfig)
	}

	ctrl.SetLogger(zap.New(zap.UseDevMode(true), zapOpts))

	// only cache these Object types a Namespace scope to reduce RBAC permission requirements
	cacheOptions := cache.Options{
		ByObject: map[client.Object]cache.ByObject{
			&corev1.Pod{}: {
				Field: fields.SelectorFromSet(fields.Set{"metadata.namespace": os.Getenv("POD_NAMESPACE")}),
			},
			&corev1.Secret{}: {
				Field: fields.SelectorFromSet(fields.Set{"metadata.namespace": os.Getenv("POD_NAMESPACE")}),
			},
			&corev1.Service{}: {
				Field: fields.SelectorFromSet(fields.Set{"metadata.namespace": os.Getenv("POD_NAMESPACE")}),
			},
			&corev1.ServiceAccount{}: {
				Field: fields.SelectorFromSet(fields.Set{"metadata.namespace": os.Getenv("POD_NAMESPACE")}),
			},
			&corev1.PersistentVolumeClaim{}: {
				Field: fields.SelectorFromSet(fields.Set{"metadata.namespace": os.Getenv("POD_NAMESPACE")}),
			},
			&corev1.ConfigMap{}: {
				Field: fields.SelectorFromSet(fields.Set{"metadata.namespace": os.Getenv("POD_NAMESPACE")}),
			},
			&appsv1.Deployment{}: {
				Field: fields.SelectorFromSet(fields.Set{"metadata.namespace": os.Getenv("POD_NAMESPACE")}),
			},
			&marketplacev1alpha1.MarketplaceConfig{}: {
				Field: fields.SelectorFromSet(fields.Set{"metadata.namespace": os.Getenv("POD_NAMESPACE")}),
			},
			&marketplacev1alpha1.RazeeDeployment{}: {
				Field: fields.SelectorFromSet(fields.Set{"metadata.namespace": os.Getenv("POD_NAMESPACE")}),
			},
			&razeev1alpha2.RemoteResource{}: {
				Field: fields.SelectorFromSet(fields.Set{"metadata.namespace": os.Getenv("POD_NAMESPACE")}),
			},
			&monitoringv1.ServiceMonitor{}: {
				Field: fields.SelectorFromSet(fields.Set{"metadata.namespace": os.Getenv("POD_NAMESPACE")}),
			},
		},
	}

	metricsOpts := metricsserver.Options{
		BindAddress: metricsAddr,
	}

	opts := ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsOpts,
		HealthProbeBindAddress: probeAddr,
		PprofBindAddress:       profBindAddress,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "8fbe3a23.marketplace.redhat.com",
		Cache:                  cacheOptions,
	}

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

	if err = (&controllers.ClusterServiceVersionReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("ClusterServiceVersion"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterServiceVersion")
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

	if err = (&controllers.RHMSubscriptionController{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("RHMSubscription"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RHMSubscription")
		os.Exit(1)
	}

	if err = (&controllers.SubscriptionReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("SubscriptionReconciler"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "SubscriptionReconciler")
		os.Exit(1)
	}

	doneChan := make(chan struct{})

	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}

	if err := mgr.AddReadyzCheck("check", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	customHandler := (&shutdownHandler{
		client: mgr.GetClient(),
	})

	setupLog.Info("starting manager")
	if err := mgr.Start(customHandler.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
	close(doneChan)
}

var onlyOneSignalHandler = make(chan struct{})
var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

type shutdownHandler struct {
	client client.Client
	cfg    *config.OperatorConfig
}

func (s *shutdownHandler) SetupSignalHandler() context.Context {
	close(onlyOneSignalHandler) // panics when called twice

	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal, 2)
	signal.Notify(c, shutdownSignals...)
	go func() {
		<-c
		setupLog.Info("shutdown signal received")
		cancel()
		<-c
		setupLog.Info("second shutdown signal received, killing")
		os.Exit(1) // second signal. Exit directly.
	}()

	return ctx
}

func (s *shutdownHandler) cleanupOperatorGroup() error {
	var operatorGroupEnvVar = "OPERATOR_GROUP"

	operatorGroupName, found := os.LookupEnv(operatorGroupEnvVar)

	if found && len(operatorGroupName) != 0 {
		setupLog.Info("operatorGroup found, attempting to update")
		operatorGroup := &olmv1.OperatorGroup{}
		subscription := &olmv1alpha1.Subscription{}

		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			err := s.client.Get(context.TODO(),
				types.NamespacedName{Name: "redhat-marketplace-operator", Namespace: s.cfg.DeployedNamespace},
				subscription,
			)

			if !k8serrors.IsNotFound(err) {
				setupLog.Info("subscription found, not cleaning operator group")
				return nil
			}

			err = s.client.Get(context.TODO(),
				types.NamespacedName{Name: operatorGroupName, Namespace: s.cfg.DeployedNamespace},
				operatorGroup,
			)

			if err != nil && !k8serrors.IsNotFound(err) {
				return err
			} else if err == nil {
				operatorGroup.Spec.TargetNamespaces = []string{s.cfg.DeployedNamespace}
				operatorGroup.Spec.Selector = nil

				err = s.client.Update(context.TODO(), operatorGroup)
				if err != nil {
					return err
				}
			}
			return nil
		})

		if err != nil {
			setupLog.Error(err, "error updating operatorGroup")
			return err
		}
	}

	return nil
}
