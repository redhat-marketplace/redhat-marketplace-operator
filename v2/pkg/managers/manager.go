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
	"time"

	"github.com/go-logr/logr"
	"github.com/google/wire"
	"k8s.io/apimachinery/pkg/api/meta"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc

	"emperror.dev/errors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/runnables"
	"github.com/spf13/pflag"
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
		wire.FieldsOf(new(*ControllerFields), "Client", "Logger", "Scheme", "Config"),
		kubernetes.NewForConfig,
		dynamic.NewForConfig,
		wire.Bind(new(kubernetes.Interface), new(*kubernetes.Clientset)),
	)

	// ProvideConfiglessManagerSet is the same as ProvideManagerSet
	// but with no config. This allows for use of envtest
	ProvideConfiglessManagerSet = wire.NewSet(
		wire.FieldsOf(new(*ControllerFields), "Client", "Logger", "Scheme", "Config"),
		kubernetes.NewForConfig,
		dynamic.NewForConfig,
		wire.Bind(new(kubernetes.Interface), new(*kubernetes.Clientset)),
	)

	// ProvideCacheClientSet is to be used by
	// wire files to get a cached client
	ProvideCachedClientSet = wire.NewSet(
		config.GetConfig,
		kubernetes.NewForConfig,
		ProvideCachedClient,
		ProvideNewCache,
		StartCache,
		NewDynamicRESTMapper,
		dynamic.NewForConfig,
		wire.Bind(new(kubernetes.Interface), new(*kubernetes.Clientset)),
	)

	ProvideSimpleClientSet = wire.NewSet(
		config.GetConfig,
		kubernetes.NewForConfig,
		ProvideSimpleClient,
		NewDynamicRESTMapper,
		dynamic.NewForConfig,
		wire.Bind(new(kubernetes.Interface), new(*kubernetes.Clientset)),
	)
)

type OperatorName string

type ControllerMain struct {
	Name     OperatorName
	FlagSets []*pflag.FlagSet
	Manager  manager.Manager
}

type ControllerFields struct {
	Client client.Client
	Scheme *k8sruntime.Scheme
	Logger logr.Logger
	Config *rest.Config
}

var _ inject.Client = &ControllerFields{}
var _ inject.Logger = &ControllerFields{}
var _ inject.Scheme = &ControllerFields{}
var _ inject.Config = &ControllerFields{}

func (m *ControllerFields) InjectLogger(l logr.Logger) error {
	m.Logger = l
	return nil
}

func (m *ControllerFields) InjectScheme(scheme *k8sruntime.Scheme) error {
	m.Scheme = scheme
	return nil
}

func (m *ControllerFields) InjectClient(client client.Client) error {
	m.Client = client
	return nil
}

func (m *ControllerFields) InjectConfig(cfg *rest.Config) error {
	m.Config = cfg
	return nil
}

func ProvidePodMonitorConfig(namespace DeployedNamespace) runnables.PodMonitorConfig {
	return runnables.PodMonitorConfig{
		Namespace: string(namespace),
		RetryTime: 30 * time.Second,
	}
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
) (*CacheIsStarted, error) {
	errChan := make(chan error)
	stopCh := make(chan struct{})
	doneChan := make(chan bool)
	timer := time.NewTimer(time.Minute)
	defer func() {
		if !timer.Stop() {
			<-timer.C
		}
	}()
	defer close(doneChan)
	defer close(stopCh)
	defer close(errChan)

	go func() {
		log.Info("starting cache")
		err := cache.Start(ctx.Done())
		if err != nil {
			errChan <- err
			log.Error(err, "error starting cache")
			return
		}
	}()

	log.Info("cache started")

	go func() {
		log.Info("checking if cache is started")
		for !cache.WaitForCacheSync(stopCh) {
		}
		doneChan <- true
	}()

	select {
	case err := <-errChan:
		return nil, err
	case <-doneChan:
		log.Info("Cache has synced")
		return &CacheIsStarted{}, nil
	case <-timer.C:
		return nil, errors.New("Timed out while starting cache")
	}
}

func ProvideCachedClient(
	c *rest.Config,
	mapper meta.RESTMapper,
	scheme *k8sruntime.Scheme,
	inCache cache.Cache,
	options ClientOptions,
) (client.Client, error) {
	writeObj, err := newCachedClient(inCache, c, client.Options{Scheme: scheme, Mapper: mapper})
	if err != nil {
		return nil, err
	}

	if options.DryRunClient {
		writeObj = client.NewDryRunClient(writeObj)
	}

	return writeObj, nil
}

func ProvideSimpleClient(
	c *rest.Config,
	mapper meta.RESTMapper,
	scheme *k8sruntime.Scheme,
	options ClientOptions,
) (client.Client, error) {
	writeObj, err := newSimpleClient(c, client.Options{Scheme: scheme, Mapper: mapper})
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

func ProvideManagerClient(mgr manager.Manager) client.Client {
	return mgr.GetClient()
}

func NewDynamicRESTMapper(cfg *rest.Config) (meta.RESTMapper, error) {
	return apiutil.NewDynamicRESTMapper(cfg)
}

// newCachedClient creates the default caching client
func newCachedClient(ca cache.Cache, config *rest.Config, options client.Options) (client.Client, error) {
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

// newSimpleClient creates a new client
func newSimpleClient(config *rest.Config, options client.Options) (client.Client, error) {
	c, err := client.New(config, options)
	if err != nil {
		return nil, err
	}

	return c, nil
}
