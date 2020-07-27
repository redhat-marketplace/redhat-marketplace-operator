package metric_generator

import (
	"compress/gzip"
	"context"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/managers"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/logger"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	metricsstore "k8s.io/kube-state-metrics/pkg/metrics_store"
	"k8s.io/kube-state-metrics/pkg/options"
	"k8s.io/kube-state-metrics/pkg/util/proc"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	metricsPath = "/metrics"
	healthzPath = "/healthz"
)

var log = logger.NewLogger("meteric_generator")

var collectorStop = StopCollector(make(chan struct{}))

type Service struct {
	k8sclient client.Client
	opts      *options.Options
	collector *Collector
}

func (s *Service) Serve(done chan struct{}) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := s.collector.Register()

	if err != nil {
		return err
	}

	opts := s.opts
	storeBuilder := NewBuilder()
	metricsRegistry := prometheus.NewRegistry()
	storeBuilder.WithNamespaces(options.DefaultNamespaces)

	proc.StartReaper()

	storeBuilder.WithKubeClient(s.k8sclient)
	storeBuilder.WithSharding(opts.Shard, opts.TotalShards)
	storeBuilder.WithCollector(s.collector)

	go s.collector.Run()

	metricsRegistry.MustRegister(
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
		prometheus.NewGoCollector(),
	)
	go telemetryServer(metricsRegistry, s.opts.TelemetryHost, s.opts.TelemetryPort)

	serveMetrics(ctx, storeBuilder, s.opts, s.opts.Host, opts.Port, s.opts.EnableGZIPEncoding)
	return nil
}

func getClientOptions() managers.ClientOptions {
	return managers.ClientOptions{
		Namespace:    "",
		DryRunClient: false,
	}
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

func serveMetrics(ctx context.Context, storeBuilder *Builder, opts *options.Options, host string, port int, enableGZIPEncoding bool) {
	// Address to listen on for web interface and telemetry
	listenAddress := net.JoinHostPort(host, strconv.Itoa(port))

	log.Info("Starting metrics server", "listenAddress", listenAddress)

	mux := http.NewServeMux()
	stores := storeBuilder.Build()

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
	stores             []*metricsstore.MetricsStore
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
