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

package metric_server

import (
	"compress/gzip"
	"context"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	opsrcv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	marketplaceredhatcomv1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/internal/metrics"
	rhmclient "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/client"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/managers"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	"github.com/sasha-s/go-deadlock"
	"k8s.io/apimachinery/pkg/runtime"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/openshift/origin/pkg/util/proc"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/meter_definition"
	corev1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kube-state-metrics/pkg/options"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	metricsPath = "/metrics"
	healthzPath = "/healthz"
)

var log = logf.Log.WithName("meteric_generator")
var reg = prometheus.NewRegistry()

type Service struct {
	k8sclient        client.Client
	k8sRestClient    clientset.Interface
	opts             *options.Options
	cache            cache.Cache
	metricsRegistry  *prometheus.Registry
	cc               reconcileutils.ClientCommandRunner
	meterDefStore    *meter_definition.MeterDefinitionStoreBuilder
	statusProcessor  *meter_definition.StatusProcessor
	serviceProcessor *meter_definition.ServiceProcessor
	isCacheStarted   managers.CacheIsStarted

	mutex deadlock.Mutex `wire:"-"`
}

func (s *Service) Serve(done <-chan struct{}) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts := s.opts
	storeBuilder := metrics.NewBuilder()
	storeBuilder.WithNamespaces(options.DefaultNamespaces)

	proc.StartReaper()

	s.meterDefStore.SetNamespaces(options.DefaultNamespaces)
	stores := s.meterDefStore.CreateStores()

	storeBuilder.WithContext(ctx)
	storeBuilder.WithKubeClient(s.k8sRestClient)
	storeBuilder.WithClientCommand(s.cc)
	storeBuilder.WithMeterDefinitionStores(stores)

	for expectedType, store := range stores {
		log.Info("stores", "type", expectedType, "store", store)
		p := s.statusProcessor.New(store)
		go func() {
			err := p.Start(ctx)
			log.Error(err, "failed to register status processor")

			panic(err)
		}()
	}

	store := stores[meter_definition.ServiceStore]
	log.Info("service stores", "store", store)
	p := s.serviceProcessor.New(store)

	go func() {
		err := p.Start(ctx)
		log.Error(err, "failed to register service processor")
		panic(err)
	}()

	s.metricsRegistry.MustRegister(
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
		prometheus.NewGoCollector(),
	)
	go telemetryServer(s.metricsRegistry, s.opts.TelemetryHost, s.opts.TelemetryPort)

	serveMetrics(ctx, storeBuilder, s.opts, s.opts.Host, opts.Port, s.opts.EnableGZIPEncoding)
	return nil
}

func getClientOptions() managers.ClientOptions {
	return managers.ClientOptions{
		Namespace:    "",
		DryRunClient: false,
	}
}

func addIndex(
	ctx context.Context,
	cache cache.Cache) (managers.CacheIsIndexed, error) {

	err := rhmclient.AddOperatorSourceIndex(cache)
	if err != nil {
		log.Error(err, "")
		return managers.CacheIsIndexed{}, err
	}

	err = rhmclient.AddOwningControllerIndex(cache,
		[]runtime.Object{
			&corev1.Pod{},
			&corev1.Service{},
			&corev1.PersistentVolumeClaim{},
			&monitoringv1.ServiceMonitor{},
		})

	if err != nil {
		log.Error(err, "")
		return managers.CacheIsIndexed{}, err
	}

	err = rhmclient.AddUIDIndex(cache,
		[]runtime.Object{
			&corev1.Pod{},
			&corev1.Service{},
			&corev1.PersistentVolumeClaim{},
			&marketplacev1alpha1.MeterDefinition{},
			&monitoringv1.ServiceMonitor{},
		})

	if err != nil {
		log.Error(err, "")
		return managers.CacheIsIndexed{}, err
	}

	return managers.CacheIsIndexed{}, nil
}

func provideRegistry() *prometheus.Registry {
	return prometheus.NewRegistry()
}

func provideContext() context.Context {
	return context.Background()
}

func telemetryServer(registry prometheus.Gatherer, host string, port int) {
	// Address to listen on for web interface and telemetry
	listenAddress := net.JoinHostPort(host, strconv.Itoa(port))

	log.Info("Starting kube-state-metrics self metrics server", "listenAddress", listenAddress)

	mux := http.NewServeMux()

	// Add metricsPath
	mux.Handle(metricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{ErrorLog: promLogger{}}))
	// Add index
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>RHM-Metering-Metrics Metrics Server</title></head>
             <body>
             <h1>RHM-Metering-Metrics Metrics</h1>
			 <ul>
             <li><a href='` + metricsPath + `'>metrics</a></li>
			 </ul>
             </body>
             </html>`))
	})

	err := http.ListenAndServe(listenAddress, mux)
	if err != nil {
		log.Error(err, "failing to listen and serve")
		panic(err)
	}
}

func serveMetrics(ctx context.Context, storeBuilder *metrics.Builder, opts *options.Options, host string, port int, enableGZIPEncoding bool) {
	// Address to listen on for web interface and telemetry
	listenAddress := net.JoinHostPort(host, strconv.Itoa(port))

	log.Info("Starting metrics server", "listenAddress", listenAddress)

	mux := http.NewServeMux()
	stores := storeBuilder.Build()

	for _, store := range stores {
		store.Start(ctx)
	}

	log.Info("built stores")

	m := &metricHandler{stores, enableGZIPEncoding}
	mux.Handle(metricsPath, m)

	// Add healthzPath
	mux.HandleFunc(healthzPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(http.StatusText(http.StatusOK)))
	})
	// Add index
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>RHM-Metering-Metrics Server</title></head>
             <body>
             <h1>RHM-Metering-Metrics Metrics</h1>
			 <ul>
             <li><a href='` + metricsPath + `'>metrics</a></li>
             <li><a href='` + healthzPath + `'>healthz</a></li>
			 </ul>
             </body>
             </html>`))
	})
	err := http.ListenAndServe(listenAddress, mux)
	if err != nil {
		log.Error(err, "failing to listen and serve")
		panic(err)
	}
}

type metricHandler struct {
	stores             []*metrics.MetricsStore
	enableGZIPEncoding bool
}

func (m *metricHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	resHeader := w.Header()
	var writer io.Writer = w

	// Set the exposition format version of Prometheus.
	// https://prometheus.io/docs/instrumenting/exposition_formats/#text-based-format
	resHeader.Set("Content-Type", `text/plain; version=`+"0.0.4")

	if m.enableGZIPEncoding {
		// Gzip response if requested. Taken from
		// github.com/prometheus/client_golang/prometheus/promhttp.decorateWriter.
		reqHeader := r.Header.Get("Accept-Encoding")
		parts := strings.Split(reqHeader, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "gzip" || strings.HasPrefix(part, "gzip;") {
				writer = gzip.NewWriter(writer)
				resHeader.Set("Content-Encoding", "gzip")
			}
		}
	}

	for _, c := range m.stores {
		c.WriteAll(w)
	}

	// In case we gzipped the response, we have to close the writer.
	if closer, ok := writer.(io.Closer); ok {
		closer.Close()
	}
}

func provideScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(marketplaceredhatcomv1alpha1.AddToScheme(scheme))
	utilruntime.Must(openshiftconfigv1.AddToScheme(scheme))
	utilruntime.Must(olmv1.AddToScheme(scheme))
	utilruntime.Must(opsrcv1.AddToScheme(scheme))
	utilruntime.Must(olmv1alpha1.AddToScheme(scheme))
	utilruntime.Must(monitoringv1.AddToScheme(scheme))
	return scheme
}
