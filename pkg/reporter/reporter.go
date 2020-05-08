package reporter

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/meirf/gopart"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	k8sconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	metricsHost            = "0.0.0.0"
	metricsPort      int32 = 8383
	additionalLabels       = []model.LabelName{"pod", "namespace", "service"}
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
	api             v1.API
	log             logr.Logger
	debug           logr.InfoLogger
	maxRoutines     int
	queryResults    chan model.Value
	mgr             manager.Manager
	outputDirectory string
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
		Address: "https://localhost:9090", //TODO: replace with https
	})

	if err != nil {
		return nil, err
	}

	v1api := v1.NewAPI(client)

	logger := logf.Log.WithName("reporter")

	return &MarketplaceReporter{
		api:             v1api,
		log:             logger,
		debug:           logger.V(3),
		mgr:             mgr,
		outputDirectory: ".",
	}, nil
}

func (r *MarketplaceReporter) GetMeterReport() (*marketplacev1alpha1.MeterReport, error) {
	return nil, nil
}

func (r *MarketplaceReporter) CollectMetrics(
	report *marketplacev1alpha1.MeterReport,
	meterDefinitions []*marketplacev1alpha1.MeterDefinition,
) (map[MetricKey]MetricBase, error) {
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

	go r.Process(promModelsChan, resultsMap, &resultsMapMutex, report, processDone, errorsChan)

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
			// TODO: use metadata to build a smart roll up
			// Guage = delta
			// Counter = increase
			// Histogram and summary are unsupported
			query := &PromQuery{
				Metric: metric,
				Labels: map[string]string{
					"meter_domain": mdef.Spec.MeterDomain,
					"meter_kind":   mdef.Spec.MeterKind,
					//"meter_version" : mdef.Spec.MeterVersion,
				},
				Functions: []string{"increase"},
				Time:      "60m",
				Start:     startTime,
				End:       endTime,
				Step:      time.Hour,
				SumBy:     []string{"meter_kind", "meter_domain", "meter_version", "pod", "namespace"},
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
	report *marketplacev1alpha1.MeterReport,
	done chan<- bool,
	errors chan<- error,
) {
	syncProcess := func(
		name string,
		mdef *marketplacev1alpha1.MeterDefinition,
		report *marketplacev1alpha1.MeterReport,
		m model.Value,
	) {
		//# do the work
		switch m.Type() {
		case model.ValMatrix:
			matrixVals := m.(model.Matrix)

			for _, matrix := range matrixVals {
				r.debug.Info("adding metric", "metric", matrix.Metric)

				for _, pair := range matrix.Values {
					func() {
						key := MetricKey{
							ReportPeriodStart: report.Spec.StartTime.Format(time.RFC3339),
							ReportPeriodEnd:   report.Spec.EndTime.Format(time.RFC3339),
							IntervalStart:     pair.Timestamp.Time().Format(time.RFC3339),
							IntervalEnd:       pair.Timestamp.Add(time.Hour).Time().Format(time.RFC3339),
							MeterDomain:       mdef.Spec.MeterDomain,
							MeterKind:         mdef.Spec.MeterKind,
							MeterVersion:      mdef.Spec.MeterVersion,
						}

						mutex.Lock()
						defer mutex.Unlock()

						base, ok := results[key]

						if !ok {
							base = MetricBase{
								Key: key,
							}
						}

						r.debug.Info("adding pair", "metric", matrix.Metric, "pair", pair)
						metricPairs := []interface{}{name, pair.Value.String()}
						metricPairs = append(metricPairs, getKeysFromMetric(matrix.Metric, additionalLabels)...)
						err := base.AddMetrics(metricPairs...)

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
				syncProcess(pmodel.MetricName, pmodel.MeterDefinition, report, pmodel.Value)
			}
		}()
	}
	wg.Wait()
	done <- true
}

func (r *MarketplaceReporter) WriteReport(
	source uuid.UUID,
	marketplaceConfig *marketplacev1alpha1.MarketplaceConfig,
	report *marketplacev1alpha1.MeterReport,
	metrics map[MetricKey]MetricBase,
) ([]string, error) {

	metadata := NewReportMetadata(source, ReportSourceMetadata{
		RhmAccountID: marketplaceConfig.Spec.RhmAccountID,
		RhmClusterID: marketplaceConfig.Spec.ClusterUUID,
	})

	const partitionSize = 100

	metricsArr := make([]*MetricBase, 0, len(metrics))

	for _, v := range metrics {
		metricsArr = append(metricsArr, &v)
	}

	filenames := []string{}

	for idxRange := range gopart.Partition(len(metricsArr), partitionSize) {
		metricReport := NewReport()
		metadata.AddMetricsReport(metricReport)
		metricReport.AddMetrics(metricsArr[idxRange.Low:idxRange.High]...)
		metadata.UpdateMetricsReport(metricReport)

		marshallBytes, err := json.Marshal([]*MetricsReport{metricReport})
		r.debug.Info(string(marshallBytes))
		if err != nil {
			r.log.Error(err, "failed to marshal metrics report", "report", metricReport)
			return nil, err
		}
		filename := filepath.Join(
			r.outputDirectory,
			fmt.Sprintf("%s.json", metricReport.ReportSliceID.String()))
		err = ioutil.WriteFile(
			filename,
			marshallBytes,
			0644)

		if err != nil {
			r.log.Error(err, "failed to write file", "file", filename)
			return nil, err
		}

		filenames = append(filenames, filename)
	}

	marshallBytes, err := json.Marshal(metadata)
	if err != nil {
		r.log.Error(err, "failed to marshal report metadata", "metadata", metadata)
		return nil, err
	}

	filename := filepath.Join(
		r.outputDirectory,
		"metadata.json")
	err = ioutil.WriteFile(
		filename,
		marshallBytes,
		0644)
	if err != nil {
		r.log.Error(err, "failed to write file", "file", filename)
		return nil, err
	}

	filenames = append(filenames, filename)

	return filenames, nil
}

func getKeysFromMetric(metric model.Metric, labels []model.LabelName) []interface{} {
	allLabels := make([]interface{}, 0, len(labels)*2)
	for _, label := range labels {
		if val, ok := metric[label]; ok {
			allLabels = append(allLabels, string(label), string(val))
		}
	}
	return allLabels
}
