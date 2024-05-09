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

package server

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	opsrcv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	marketplaceredhatcomv1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"

	marketplaceredhatcomv1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/internal/metrics"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/engine"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/managers"
	"k8s.io/apimachinery/pkg/runtime"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	k8sapiflag "k8s.io/component-base/cli/flag"

	goruntime "runtime"

	"github.com/gotidy/ptr"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
)

const (
	metricsPath = "/metrics"
	healthzPath = "/healthz"
	readyzPath  = "/readyz"
)

var log = logf.Log.WithName("meteric_generator")

type Service struct {
	opts            *Options
	metricsRegistry *prometheus.Registry
	engine          *engine.Engine
	prometheusData  *metrics.PrometheusData
	cluster         cluster.Cluster

	*isReady `wire:"-"`
}

type isReady struct {
	ready bool
	m     sync.Mutex
}

func (i *isReady) MarkReady() {
	i.m.Lock()
	defer i.m.Unlock()

	i.ready = true
}

func (i *isReady) IsReady() bool {
	i.m.Lock()
	defer i.m.Unlock()

	return i.ready
}

func (s *Service) Serve(done <-chan struct{}) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.isReady = &isReady{}

	go func() {
		err := s.engine.Start(ctx)

		if err != nil {
			log.Error(err, "failed to start engine")
			return
		}

		s.isReady.MarkReady()
	}()

	s.serveMetrics()
	return nil
}

func getClientOptions() managers.ClientOptions {
	return managers.ClientOptions{
		Namespace:    "",
		DryRunClient: false,
		ByObject: map[client.Object]cache.ByObject{
			&corev1.Pod{}:                   cache.ByObject{UnsafeDisableDeepCopy: ptr.Bool(true)},
			&corev1.Service{}:               cache.ByObject{UnsafeDisableDeepCopy: ptr.Bool(true)},
			&corev1.PersistentVolume{}:      cache.ByObject{UnsafeDisableDeepCopy: ptr.Bool(true)},
			&corev1.PersistentVolumeClaim{}: cache.ByObject{UnsafeDisableDeepCopy: ptr.Bool(true)},
		},

		/*
			DisableDeepCopyByObject: cache.DisableDeepCopyByObject{
				&corev1.Pod{}:                   true,
				&corev1.Service{}:               true,
				&corev1.PersistentVolume{}:      true,
				&corev1.PersistentVolumeClaim{}: true,
			},
		*/
	}
}

func provideRegistry() *prometheus.Registry {
	return prometheus.NewRegistry()
}

func provideContext() context.Context {
	return context.Background()
}

func provideCluster(restConfig *rest.Config, scheme *runtime.Scheme) (cluster.Cluster, error) {
	return cluster.New(restConfig, func(o *cluster.Options) { o.Scheme = scheme })
}

/*
It could be novel to use controller-utils metrics server
https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.0/pkg/metrics/server
and register metrics on the global prometheus registry
*/

//nolint:errcheck
func (s *Service) serveMetrics() {

	// validate TLS flags
	tlsMV, err := k8sapiflag.TLSVersion(s.opts.TLSMinVersion)
	if err != nil {
		log.Error(err, "tls-min-version TLS min version invalid", "flag", "tls-min-version")
		panic(err)
	}

	tlsCS, err := k8sapiflag.TLSCipherSuites(s.opts.TLSCipherSuites)
	if err != nil {
		log.Error(err, "failed to convert TLS cipher suite name to ID", "flag", "tls-cipher-suites")
		panic(err)
	}

	// Setup Context
	ctx := ctrl.SetupSignalHandler()

	// Initialize a new cert watcher with cert/key pair
	watcher, err := certwatcher.New(s.opts.TLSCert, s.opts.TLSKey)
	if err != nil {
		log.Error(err, "failed to watch TLS certificate and key", "flag", "tls-cert", "flag", "tls-key")
		panic(err)
	}

	// Start goroutine with certwatcher running fsnotify against supplied certdir
	go func() {
		if err := watcher.Start(ctx); err != nil {
			log.Error(err, "failed to start certificate watcher")
			panic(err)
		}
	}()

	// Setup TLS listener using GetCertficate for fetching the cert when changes
	listenAddress := net.JoinHostPort(s.opts.Host, strconv.Itoa(s.opts.Port))
	listener, err := tls.Listen("tcp", listenAddress, &tls.Config{
		GetCertificate: watcher.GetCertificate,
		MinVersion:     tlsMV,
		CipherSuites:   tlsCS,
	})
	if err != nil {
		log.Error(err, "failed to listen", "address", listenAddress)
		panic(err)
	}

	// Address to listen on for web interface and telemetry
	log.Info("Starting metrics server", "listenAddress", listenAddress)

	// use auth filter
	authFilter, err := filters.WithAuthenticationAndAuthorization(s.cluster.GetConfig(), s.cluster.GetHTTPClient())
	if err != nil {
		log.Error(err, "failed to initialize authentication filter")
		panic(err)
	}

	mux := http.NewServeMux()

	// Add pprof if debug is enabled
	if debug := os.Getenv("PPROF_DEBUG"); debug == trueStr {
		goruntime.SetMutexProfileFraction(5)
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}

	// wrap metricsPath handler with auth filter
	m, err := authFilter(log, &metricHandler{s.prometheusData, s.opts.EnableGZIPEncoding})
	if err != nil {
		log.Error(err, "failed to wrap metrics handler with authentication filter")
		panic(err)
	}
	mux.Handle(metricsPath, m)

	// Add healthzPath
	mux.HandleFunc(healthzPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(http.StatusText(http.StatusOK)))
	})

	// Add readyzPath
	mux.HandleFunc(readyzPath, func(w http.ResponseWriter, r *http.Request) {
		if s.isReady.IsReady() {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(http.StatusText(http.StatusOK)))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(http.StatusText(http.StatusServiceUnavailable)))
		}
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
             <li><a href='` + readyzPath + `'>healthz</a></li>
			 </ul>
             </body>
             </html>`))
	})

	// Initialize tls server
	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Start goroutine for handling server shutdown.
	go func() {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			panic(err)
		}
	}()

	// Serve
	if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Error(err, "failed to serve http")
		panic(err)
	}

}

type metricHandler struct {
	stores             *metrics.PrometheusData
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

	err := m.stores.WriteAll(w)
	if err != nil {
		log.Error(err, "failed to write stores")
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
	utilruntime.Must(opsrcv1.AddToScheme(scheme))
	utilruntime.Must(olmv1alpha1.AddToScheme(scheme))
	utilruntime.Must(monitoringv1.AddToScheme(scheme))
	utilruntime.Must(marketplaceredhatcomv1beta1.AddToScheme(scheme))
	return scheme
}
