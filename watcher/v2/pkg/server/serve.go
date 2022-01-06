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
	"context"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"strconv"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	opsrcv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	marketplaceredhatcomv1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"

	marketplaceredhatcomv1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/pkg/engine"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/managers"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	"k8s.io/apimachinery/pkg/runtime"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	goruntime "runtime"

	"github.com/sasha-s/go-deadlock"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kube-state-metrics/pkg/options"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	//	metricsPath = "/metrics"
	healthzPath = "/healthz"
)

var log = logf.Log.WithName("watcher")
var reg = prometheus.NewRegistry()

type Service struct {
	k8sclient     client.Client
	k8sRestClient clientset.Interface
	opts          *options.Options
	cache         cache.Cache
	cc            reconcileutils.ClientCommandRunner
	indexed       managers.CacheIsIndexed
	started       managers.CacheIsStarted
	razeeengine   *engine.RazeeEngine

	mutex deadlock.Mutex `wire:"-"`
}

func (s *Service) Serve(done <-chan struct{}) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := s.razeeengine.Start(ctx)

	if err != nil {
		log.Error(err, "failed to start razee engine")
		panic(err)
	}

	go telemetryServer(s.opts.TelemetryHost, s.opts.TelemetryPort)

	s.serveMetrics(ctx, s.opts, s.opts.Host, s.opts.Port, s.opts.EnableGZIPEncoding)
	return nil
}

func getClientOptions() managers.ClientOptions {
	return managers.ClientOptions{
		Namespace:    "",
		DryRunClient: false,
	}
}

func provideRegistry() *prometheus.Registry {
	return prometheus.NewRegistry()
}

func provideContext() context.Context {
	return context.Background()
}

func telemetryServer(host string, port int) {
	// Address to listen on for web interface and telemetry
	listenAddress := net.JoinHostPort(host, strconv.Itoa(port))

	log.Info("Starting kube-state-metrics self metrics server", "listenAddress", listenAddress)

	mux := http.NewServeMux()

	// if debug enabled
	if debug := os.Getenv("PPROF_DEBUG"); debug == "true" {
		goruntime.SetMutexProfileFraction(5)
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}

	err := http.ListenAndServe(listenAddress, mux)
	if err != nil {
		log.Error(err, "failing to listen and serve")
		panic(err)
	}
}

func (s *Service) serveMetrics(ctx context.Context, opts *options.Options, host string, port int, enableGZIPEncoding bool) {
	// Address to listen on for web interface and telemetry
	listenAddress := net.JoinHostPort(host, strconv.Itoa(port))

	log.Info("Starting metrics server", "listenAddress", listenAddress)

	mux := http.NewServeMux()

	// Add healthzPath
	mux.HandleFunc(healthzPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(http.StatusText(http.StatusOK)))
	})

	err := http.ListenAndServe(listenAddress, mux)
	if err != nil {
		log.Error(err, "failing to listen and serve")
		panic(err)
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
	utilruntime.Must(marketplaceredhatcomv1beta1.AddToScheme(scheme))
	return scheme
}
