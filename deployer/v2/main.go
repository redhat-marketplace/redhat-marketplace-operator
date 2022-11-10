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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	opsrcv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"net/http"
	"net/http/pprof"
	_ "net/http/pprof"

	osappsv1 "github.com/openshift/api/apps/v1"
	controllers "github.com/redhat-marketplace/redhat-marketplace-operator/deployer/v2/controllers/marketplace"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/inject"
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
	mktypes.RegisterImageStream(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
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
	newCacheFunc := cache.BuilderWithOptions(cache.Options{
		SelectorsByObject: cache.SelectorsByObject{
			&corev1.Pod{}: {
				Field: fields.SelectorFromSet(fields.Set{"metadata.namespace": os.Getenv("POD_NAMESPACE")}),
			},
			&corev1.Secret{}: {
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
			&batchv1.Job{}: {
				Field: fields.SelectorFromSet(fields.Set{"metadata.namespace": os.Getenv("POD_NAMESPACE")}),
			},
			&marketplacev1alpha1.MarketplaceConfig{}: {
				Field: fields.SelectorFromSet(fields.Set{"metadata.namespace": os.Getenv("POD_NAMESPACE")}),
			},
			&marketplacev1alpha1.RazeeDeployment{}: {
				Field: fields.SelectorFromSet(fields.Set{"metadata.namespace": os.Getenv("POD_NAMESPACE")}),
			},
			&marketplacev1alpha1.RemoteResourceS3{}: {
				Field: fields.SelectorFromSet(fields.Set{"metadata.namespace": os.Getenv("POD_NAMESPACE")}),
			},
		},
	})

	opts := ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "8fbe3a23.marketplace.redhat.com",
		NewCache:               newCacheFunc,
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
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	injector, err := inject.ProvideInjector(mgr)
	if err != nil {
		setupLog.Error(err, "unable to inject manager")
		os.Exit(1)
	}

	if err = (&controllers.RazeeDeploymentReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("RazeeDeployment"),
		Scheme: mgr.GetScheme(),
	}).Inject(injector).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RazeeDeployment")
		os.Exit(1)
	}

	if err = (&controllers.RemoteResourceS3Reconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("RemoteResourceS3"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RemoteResourceS3")
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

	// if debug enabled
	if debug := os.Getenv("PPROF_DEBUG"); debug == "true" {
		r := http.NewServeMux()
		r.HandleFunc("/debug/pprof/", pprof.Index)
		r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		r.HandleFunc("/debug/pprof/profile", pprof.Profile)
		r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		r.HandleFunc("/debug/pprof/trace", pprof.Trace)

		if err := mgr.AddMetricsExtraHandler("/", r); err != nil {
			setupLog.Error(err, "unable to set up pprof")
			os.Exit(1)
		}
	}

	customHandler := (&shutdownHandler{
		client: mgr.GetClient(),
	}).Inject(injector)

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

func (s *shutdownHandler) Inject(injector mktypes.Injectable) *shutdownHandler {
	injector.SetCustomFields(s)
	return s
}

func (s *shutdownHandler) InjectOperatorConfig(cfg *config.OperatorConfig) error {
	s.cfg = cfg
	return nil
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
