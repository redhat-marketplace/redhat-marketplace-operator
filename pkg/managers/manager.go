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

package managers

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/wire"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	kubemetrics "github.com/operator-framework/operator-sdk/pkg/kube-metrics"
	sdkVersion "github.com/operator-framework/operator-sdk/version"
	"k8s.io/apimachinery/pkg/api/meta"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc
	"emperror.dev/errors"
	"github.com/operator-framework/operator-sdk/pkg/leader"
	"github.com/operator-framework/operator-sdk/pkg/log/zap"
	"github.com/operator-framework/operator-sdk/pkg/metrics"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/controller"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/managers/runnables"
	"github.com/redhat-marketplace/redhat-marketplace-operator/version"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	metricsHost               = "0.0.0.0"
	metricsPort         int32 = 8383
	operatorMetricsPort int32 = 8686
)

var (
	log = logf.Log.WithName("cmd")
	// ProvideManagerSet is to be used by
	// wire files to get a controller manager
	ProvideManagerSet = wire.NewSet(
		config.GetConfig,
		kubernetes.NewForConfig,
		discovery.NewDiscoveryClientForConfig,
		ProvideManager,
		ProvideScheme,
		ProvideManagerClient,
		dynamic.NewForConfig,
		wire.Bind(new(kubernetes.Interface), new(*kubernetes.Clientset)),
	)
	// ProvideConfiglessManagerSet is the same as ProvideManagerSet
	// but with no config. This allows for use of envtest
	ProvideConfiglessManagerSet = wire.NewSet(
		kubernetes.NewForConfig,
		ProvideManager,
		ProvideScheme,
		ProvideManagerClient,
		dynamic.NewForConfig,
		wire.Bind(new(kubernetes.Interface), new(*kubernetes.Clientset)),
	)
	// ProvideCacheClientSet is to be used by
	// wire files to get a cached client
	ProvideCachedClientSet = wire.NewSet(
		config.GetConfig,
		kubernetes.NewForConfig,
		ProvideClient,
		ProvideNewCache,
		StartCache,
		ProvideScheme,
		NewDynamicRESTMapper,
		dynamic.NewForConfig,
		wire.Bind(new(kubernetes.Interface), new(*kubernetes.Clientset)),
	)
)

func printVersion() {
	log.Info(fmt.Sprintf("Operator Version: %s", version.Version))
	log.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
	log.Info(fmt.Sprintf("Version of operator-sdk: %v", sdkVersion.Version))
}

type OperatorName string

type ControllerMain struct {
	Name        OperatorName
	FlagSets    []*pflag.FlagSet
	Controllers []controller.AddController
	Manager     manager.Manager
	PodMonitor  *runnables.PodMonitor
}

func (m *ControllerMain) ParseFlags() {
	// adding controller flags
	for _, flags := range m.FlagSets {
		pflag.CommandLine.AddFlagSet(flags)
	}

	// adding controller flags
	for _, controller := range m.Controllers {
		pflag.CommandLine.AddFlagSet(controller.FlagSet())
	}

	pflag.CommandLine.AddFlagSet(zap.FlagSet())

	pflag.Parse()

	// adding viper so we can get our flags without having to pass it down
	err := viper.BindPFlags(pflag.CommandLine)

	// Check if viper has boud flags properly
	// If not exit with "Cancelled" code
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}
}

func (m *ControllerMain) Run(stop <-chan struct{}) {
	logf.SetLogger(zap.Logger())

	printVersion()

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	ctx := context.TODO()
	// Become the leader before proceeding
	err = leader.Become(ctx, (string)(m.Name))
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	mgr := m.Manager

	log.Info("Registering Components.")

	// Setup all Controllers
	for _, control := range m.Controllers {
		if err := control.Add(mgr); err != nil {
			log.Error(err, "")
			os.Exit(1)
		}
	}

	if m.PodMonitor != nil {
		log.Info("starting pod monitor")
		m.Manager.Add(m.PodMonitor)
	}

	// Add the Metrics Service
	go func() {
		addMetrics(ctx, cfg)
	}()

	log.Info("Starting the Cmd.")

	// Start the Cmd
	if err := mgr.Start(stop); err != nil {
		log.Error(err, "Manager exited non-zero")
		os.Exit(1)
	}
}

// addMetrics will create the Services and Service Monitors to allow the operator export the metrics by using
// the Prometheus operator
func addMetrics(ctx context.Context, cfg *rest.Config) {
	// Get the namespace the operator is currently deployed in.
	operatorNs, err := k8sutil.GetOperatorNamespace()
	if err != nil {
		if errors.Is(err, k8sutil.ErrRunLocal) {
			log.Info("Skipping CR metrics server creation; not running in a cluster.")
			return
		}
	}

	if err := serveCRMetrics(cfg, operatorNs); err != nil {
		log.Info("Could not generate and serve custom resource metrics", "error", err.Error())
	}

	// Add to the below struct any other metrics ports you want to expose.
	servicePorts := []v1.ServicePort{
		{Port: metricsPort, Name: metrics.OperatorPortName, Protocol: v1.ProtocolTCP, TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: metricsPort}},
		{Port: operatorMetricsPort, Name: metrics.CRPortName, Protocol: v1.ProtocolTCP, TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: operatorMetricsPort}},
	}

	// Create Service object to expose the metrics port(s).
	service, err := metrics.CreateMetricsService(ctx, cfg, servicePorts)
	if err != nil {
		log.Info("Could not create metrics Service", "error", err.Error())
	}

	// CreateServiceMonitors will automatically create the prometheus-operator ServiceMonitor resources
	// necessary to configure Prometheus to scrape metrics from this operator.
	services := []*v1.Service{service}

	// The ServiceMonitor is created in the same namespace where the operator is deployed
	_, err = metrics.CreateServiceMonitors(cfg, operatorNs, services)
	if err != nil {
		log.Info("Could not create ServiceMonitor object", "error", err.Error())
		// If this operator is deployed to a cluster without the prometheus-operator running, it will return
		// ErrServiceMonitorNotPresent, which can be used to safely skip ServiceMonitor creation.
		if err == metrics.ErrServiceMonitorNotPresent {
			log.Info("Install prometheus-operator in your cluster to create ServiceMonitor objects", "error", err.Error())
		}
	}
}

// serveCRMetrics gets the Operator/CustomResource GVKs and generates metrics based on those types.
// It serves those metrics on "http://metricsHost:operatorMetricsPort".
func serveCRMetrics(cfg *rest.Config, operatorNs string) error {
	// The function below returns a list of filtered operator/CR specific GVKs. For more control, override the GVK list below
	// with your own custom logic. Note that if you are adding third party API schemas, probably you will need to
	// customize this implementation to avoid permissions issues.
	filteredGVK, err := k8sutil.GetGVKsFromAddToScheme(apis.AddToScheme)
	if err != nil {
		return err
	}

	// The metrics will be generated from the namespaces which are returned here.
	// NOTE that passing nil or an empty list of namespaces in GenerateAndServeCRMetrics will result in an error.
	ns, err := kubemetrics.GetNamespacesForMetrics(operatorNs)
	if err != nil {
		return err
	}

	// Generate and serve custom resource specific metrics.
	err = kubemetrics.GenerateAndServeCRMetrics(cfg, ns, filteredGVK, metricsHost, operatorMetricsPort)
	if err != nil {
		return err
	}
	return nil
}

func ProvideManager(
	cfg *rest.Config,
	kscheme *k8sruntime.Scheme,
	localSchemes controller.LocalSchemes,
	opts *manager.Options,
) (manager.Manager, error) {
	if err := apis.AddToScheme(kscheme); err != nil {
		log.Error(err, "failed to add scheme")
		return nil, errors.Wrap(err, "could not add apis to scheme")
	}

	for _, apiScheme := range localSchemes {
		log.Info("adding scheme", "scheme", apiScheme.Name)
		if err := apiScheme.AddToScheme(kscheme); err != nil {
			log.Error(err, "failed to add scheme")
			return nil, errors.Wrap(err, "failed to add scheme")
		}
	}

	if opts == nil {
		return nil, errors.New("opts not provided")
	}

	// Create a new Cmd to provide shared dependencies and start components
	return manager.New(cfg, *opts)
}

type ClientOptions struct {
	SyncPeriod   *time.Duration
	DryRunClient bool
	Namespace    string
}

type CacheIsStarted struct{}
type CacheIsIndexed struct{}

func StartCache(
	ctx context.Context,
	cache cache.Cache,
	log logr.Logger,
	isIndexed CacheIsIndexed,
) CacheIsStarted {
	go func() {
		err := cache.Start(ctx.Done())
		log.Error(err, "error starting cache")
	}()
	return CacheIsStarted{}
}

func ProvideClient(
	c *rest.Config,
	mapper meta.RESTMapper,
	scheme *k8sruntime.Scheme,
	inCache cache.Cache,
	options ClientOptions,
) (client.Client, error) {
	writeObj, err := defaultNewClient(inCache, c, client.Options{Scheme: scheme, Mapper: mapper})
	if err != nil {
		return nil, err
	}

	if options.DryRunClient {
		writeObj = client.NewDryRunClient(writeObj)
	}

	return writeObj, nil
}

func ProvideNewCache(
	c *rest.Config,
	mapper meta.RESTMapper,
	scheme *k8sruntime.Scheme,
	options ClientOptions,
) (cache.Cache, error) {
	return cache.New(c,
		cache.Options{
			Scheme:    scheme,
			Mapper:    mapper,
			Resync:    options.SyncPeriod,
			Namespace: options.Namespace,
		})
}

func ProvideScheme(
	c *rest.Config,
	localSchemes controller.LocalSchemes,
) (*k8sruntime.Scheme, error) {
	scheme := k8sruntime.NewScheme()
	err := k8sscheme.AddToScheme(scheme)

	if err != nil {
		return nil, err
	}

	if err = apis.AddToScheme(scheme); err != nil {
		log.Error(err, "failed to add scheme")
		return nil, errors.Wrap(err, "could not add apis to scheme")
	}

	for _, apiScheme := range localSchemes {
		log.Info("adding scheme", "scheme", apiScheme.Name)
		if err = apiScheme.AddToScheme(scheme); err != nil {
			log.Error(err, "failed to add scheme")
			return nil, errors.Wrap(err, "failed to add scheme")
		}
	}

	return scheme, err
}

func ProvideManagerClient(mgr manager.Manager) client.Client {
	return mgr.GetClient()
}

func NewDynamicRESTMapper(cfg *rest.Config) (meta.RESTMapper, error) {
	return apiutil.NewDynamicRESTMapper(cfg)
}

// defaultNewClient creates the default caching client
func defaultNewClient(ca cache.Cache, config *rest.Config, options client.Options) (client.Client, error) {
	// Create the Client for Write operations.
	c, err := client.New(config, options)
	if err != nil {
		return nil, err
	}

	return &client.DelegatingClient{
		Reader: &client.DelegatingReader{
			CacheReader:  ca,
			ClientReader: c,
		},
		Writer:       c,
		StatusClient: c,
	}, nil
}
