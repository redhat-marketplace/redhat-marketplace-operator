package reporter

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.ibm.com/symposium/redhat-marketplace-operator/pkg/apis"
	marketplacev1alpha1 "github.ibm.com/symposium/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	k8sconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	ants "github.com/panjf2000/ants/v2"
)

var (
	metricsHost       = "0.0.0.0"
	metricsPort int32 = 8383
)

// Goals of the reporter:
// Get current meters to query
//
// Build a report for hourly since last reprot
//
// Query all the meters
//
// Break up reports into manageable chunks
//
// Upload to insights
//
// Update the CR status for each report and queue

type MarketplaceReporter struct {
	api          v1.API
	log          logr.Logger
	maxRoutines  int
	queryResults chan model.Value
	mgr          manager.Manager
}

func NewMarketplaceReporter(config *MarketplaceReporterConfig) (*MarketplaceReporter, error) {
	// Get a config to talk to the apiserver
	cfg, err := k8sconfig.GetConfig()
	if err != nil {
		return nil, err
	}

	scheme := k8sscheme.Scheme

	if err := apis.AddToScheme(scheme); err != nil {
		log.Error(err, "failed to add scheme")
		return nil, err
	}

	for _, apiScheme := range config.Schemes {
		log.Info("adding scheme", "scheme", apiScheme.Name)
		if err := apiScheme.AddToScheme(scheme); err != nil {
			log.Error(err, "failed to add scheme")
			return nil, err
		}
	}

	opts := manager.Options{
		Namespace:          config.WatchNamespace,
		MetricsBindAddress: fmt.Sprintf("%s:%d", metricsHost, metricsPort),
		Scheme:             scheme,
	}

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, opts)
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	client, err := api.NewClient(api.Config{
		Address: "https://localhost:9090", //TODO: replace
	})

	if err != nil {
		return nil, err
	}

	v1api := v1.NewAPI(client)
	return &MarketplaceReporter{
		api: v1api,
		log: logf.Log.WithName("reporter"),
		mgr: mgr,
	}, nil
}

func (r *MarketplaceReporter) GetMeterReport() (*marketplacev1alpha1.MeterReport, error) {
	return nil, nil
}

func (r *MarketplaceReporter) CollectMetrics(
	report *marketplacev1alpha1.MeterReport,
	meterDefinitions []*marketplacev1alpha1.MeterDefinition,
) (map[MetricKey]MetricBase, error) {
	defer ants.Release()
	// ctx, cancel := context.WithCancel(context.Background())
	// defer cancel()

	resultsMap := make(map[MetricKey]MetricBase)
	var resultsMapMutex sync.Mutex

	meterDefsChan := make(chan *marketplacev1alpha1.MeterDefinition, len(meterDefinitions))
	promModelsChan := make(chan meterDefPromModel)
	errorsChan := make(chan error)
	queryDone := make(chan bool, 1)
	processDone := make(chan bool, 1)

	r.log.Info("starting query")

	go r.Query(
		report.Spec.StartTime.Time,
		report.Spec.EndTime.Time,
		meterDefsChan,
		promModelsChan,
		queryDone,
		errorsChan)

	go r.Process(promModelsChan, resultsMap, &resultsMapMutex, processDone, errorsChan)

	for _, meterDef := range meterDefinitions {
		meterDefsChan <- meterDef
	}

	close(meterDefsChan)

	<-queryDone
	close(promModelsChan)

	<-processDone
	r.log.Info("processing done")


	return resultsMap, nil
}

func (r *MarketplaceReporter) GetMeterDefs(
	meterDefLabels *metav1.LabelSelector,
) ([]*marketplacev1alpha1.MeterDefinition, error) {
	return nil, nil
}

type meterDefPromModel struct {
	*marketplacev1alpha1.MeterDefinition
	model.Value
	MetricName string
}

func (r *MarketplaceReporter) Query(
	startTime, endTime time.Time,
	inMeterDefs <-chan *marketplacev1alpha1.MeterDefinition,
	outPromModels chan<- meterDefPromModel,
	done chan<- bool,
	errors chan<- error,
) {
	queryProcess := func(mdef *marketplacev1alpha1.MeterDefinition) {
		for _, metric := range mdef.Spec.ServiceMeterLabels {
			r.log.Info("query", "metric", metric)
			query := &PromQuery{
				Metric: metric,
				Labels: map[string]string{
					"meter_domain": mdef.Spec.MeterDomain,
					"meter_kind":   mdef.Spec.MeterKind,
					//"meter_version" : mdef.Spec.MeterVersion,
				},
				Functions: []string{"delta"},
				Time: "60m",
				Start: startTime,
				End:   endTime,
				Step:  time.Hour,
				SumBy: []string{ "meter_kind", "meter_domain", "meter_version", "pod", "namespace" },
			}
			r.log.Info("test", "query", query.String())
			val, warnings, err := r.queryRange(query)

			if warnings != nil {
				r.log.Info("warnings %v", warnings)
			}

			if err != nil {
				r.log.Error(err, "error encountered")
				errors <- err
			}

			outPromModels <- meterDefPromModel{mdef, val, metric}
		}
	}

	var wg sync.WaitGroup
	for w := 1; w <= 10; w++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			for mdef := range inMeterDefs {
				queryProcess(mdef)
			}
		}()
	}
	wg.Wait()
	done <- true
}

func (r *MarketplaceReporter) Process(
	inPromModels <-chan meterDefPromModel,
	results map[MetricKey]MetricBase,
	mutex *sync.Mutex,
	done chan<- bool,
	errors chan<- error,
) {
	syncProcess := func(name string, mdef *marketplacev1alpha1.MeterDefinition, m model.Value) {
		//# do the work
		switch m.Type() {
		case model.ValMatrix:
			matrixVals := m.(model.Matrix)

			for _, matrix := range matrixVals {
				r.log.V(0).Info("adding metric", "metri", matrix.Metric)
				for _, pair := range matrix.Values {
					func() {
						key := MetricKey{
							IntervalStart: pair.Timestamp.String(),
							IntervalEnd:   pair.Timestamp.Add(time.Hour).String(),
							MeterDomain:   mdef.Spec.MeterDomain,
							MeterKind:     mdef.Spec.MeterKind,
							MeterVersion:  mdef.Spec.MeterVersion,
						}

						mutex.Lock()
						defer mutex.Unlock()

						base, ok := results[key]

						if !ok {
							base = MetricBase{
								Key: key,
							}
						}

						r.log.V(0).Info("adding pair", "name", name, "", pair)
						err := base.AddMetrics(
							name, pair.Value.String(),
							"pod", matrix.Metric["pod"],
							"namespace", matrix.Metric["namespace"])

						if err != nil {
							errors <- err
						}

						results[key] = base
					}()
				}
			}
			//TODO: implement scalar, vector, etc. etc.
		}
	}

	var wg sync.WaitGroup
	for w := 1; w <= 10; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pmodel := range inPromModels {
				syncProcess(pmodel.MetricName, pmodel.MeterDefinition, pmodel.Value)
			}
		}()
	}
	wg.Wait()
	done <- true
}
