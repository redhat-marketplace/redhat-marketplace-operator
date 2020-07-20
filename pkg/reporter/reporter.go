package reporter

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/google/uuid"
	"github.com/meirf/gopart"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	loggerf "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/logger"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	additionalLabels = []model.LabelName{"pod", "namespace", "service"}
	logger           = loggerf.NewLogger("reporter")
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
	api               v1.API
	mgr               manager.Manager
	report            *marketplacev1alpha1.MeterReport
	meterDefinitions  []*marketplacev1alpha1.MeterDefinition
	prometheusService *corev1.Service
	*reporterConfig
}

type ReportName types.NamespacedName

func NewMarketplaceReporter(
	config *reporterConfig,
	mgr manager.Manager,
	report *marketplacev1alpha1.MeterReport,
	meterDefinitions []*marketplacev1alpha1.MeterDefinition,
	prometheusService *corev1.Service,
) (*MarketplaceReporter, error) {
	// Get a config to talk to the apiserver
	client, err := api.NewClient(api.Config{
		Address: "https://localhost:9090", //TODO: replace with https
	})

	if err != nil {
		return nil, err
	}

	v1api := v1.NewAPI(client)

	return &MarketplaceReporter{
		api:               v1api,
		mgr:               mgr,
		report:            report,
		meterDefinitions:  meterDefinitions,
		reporterConfig:    config,
		prometheusService: prometheusService,
	}, nil
}

func (r *MarketplaceReporter) CollectMetrics(ctx context.Context) (map[MetricKey]*MetricBase, error) {
	resultsMap := make(map[MetricKey]*MetricBase)
	var resultsMapMutex sync.Mutex

	meterDefsChan := make(chan *marketplacev1alpha1.MeterDefinition, len(r.meterDefinitions))
	promModelsChan := make(chan meterDefPromModel)
	errorsChan := make(chan error)
	queryDone := make(chan bool, 1)
	processDone := make(chan bool, 1)

	logger.Info("starting query")

	go r.query(
		r.report.Spec.StartTime.Time,
		r.report.Spec.EndTime.Time,
		meterDefsChan,
		promModelsChan,
		queryDone,
		errorsChan)

	go r.process(promModelsChan, resultsMap, &resultsMapMutex, r.report, processDone, errorsChan)

	for _, meterDef := range r.meterDefinitions {
		meterDefsChan <- meterDef
	}

	close(meterDefsChan)

	<-queryDone
	close(promModelsChan)

	<-processDone
	logger.Info("processing done")

	logger.Info("closing errors channel")
	close(errorsChan)

	errorList := []error{}

	for err := range errorsChan {
		errorList = append(errorList, err)
	}

	if len(errorList) != 0 {
		err := errors.Combine(errorList...)
		logger.Error(err, "processing errored")
		return nil, err
	}

	return resultsMap, nil
}

type meterDefPromModel struct {
	*marketplacev1alpha1.MeterDefinition
	model.Value
	MetricName string
}

func (r *MarketplaceReporter) query(
	startTime, endTime time.Time,
	inMeterDefs <-chan *marketplacev1alpha1.MeterDefinition,
	outPromModels chan<- meterDefPromModel,
	done chan<- bool,
	errorsch chan<- error,
) {
	queryProcess := func(mdef *marketplacev1alpha1.MeterDefinition) {
		for _, metric := range mdef.Spec.ServiceMeterLabels {
			logger.Info("query", "metric", metric)
			// TODO: use metadata to build a smart roll up
			// Guage = delta
			// Counter = increase
			// Histogram and summary are unsupported
			query := &PromQuery{
				Metric: metric,
				Labels: map[string]string{
					"meter_domain":  mdef.Spec.MeterDomain,
					"meter_kind":    mdef.Spec.MeterKind,
					"meter_version": mdef.Spec.MeterVersion,
				},
				Functions: []string{"increase"},
				Time:      "60m",
				Start:     startTime,
				End:       endTime,
				Step:      time.Hour,
				SumBy:     []string{"meter_kind", "meter_domain", "meter_version", "pod", "namespace"},
			}
			logger.Info("test", "query", query.String())
			val, warnings, err := r.queryRange(query)

			if warnings != nil {
				logger.Info("warnings %v", warnings)
			}

			if err != nil {
				logger.Error(err, "error encountered")
				errorsch <- err
			}

			outPromModels <- meterDefPromModel{mdef, val, metric}
		}
	}

	var wg sync.WaitGroup
	for w := 1; w <= *r.MaxRoutines; w++ {
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

func (r *MarketplaceReporter) process(
	inPromModels <-chan meterDefPromModel,
	results map[MetricKey]*MetricBase,
	mutex sync.Locker,
	report *marketplacev1alpha1.MeterReport,
	done chan<- bool,
	errorsch chan<- error,
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
				logger.Debug("adding metric", "metric", matrix.Metric)

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
							base = &MetricBase{
								Key: key,
							}
						}

						logger.Debug("adding pair", "metric", matrix.Metric, "pair", pair)
						metricPairs := []interface{}{name, pair.Value.String()}

						err := base.AddAdditionalLabels(getKeysFromMetric(matrix.Metric, additionalLabels)...)

						if err != nil {
							errorsch <- errors.Wrap(err, "failed adding additional labels")
							return
						}

						err = base.AddMetrics(metricPairs...)

						if err != nil {
							errorsch <- errors.Wrap(err, "failed adding metrics")
							return
						}

						results[key] = base
					}()
				}
			}
		case model.ValString:
		case model.ValVector:
		case model.ValScalar:
		case model.ValNone:
			errorsch <- errors.Errorf("can't process model type=%s", m.Type())
		}
	}

	var wg sync.WaitGroup
	for w := 1; w <= *r.MaxRoutines; w++ {
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
	metrics map[MetricKey]*MetricBase) ([]string, error) {
	metadata := NewReportMetadata(source, ReportSourceMetadata{
		RhmAccountID: marketplaceConfig.Spec.RhmAccountID,
		RhmClusterID: marketplaceConfig.Spec.ClusterUUID,
	})

	var partitionSize = *r.MetricsPerFile

	metricsArr := make([]*MetricBase, 0, len(metrics))

	for _, v := range metrics {
		metricsArr = append(metricsArr, v)
	}

	filenames := []string{}

	for idxRange := range gopart.Partition(len(metricsArr), partitionSize) {
		metricReport := NewReport()
		metadata.AddMetricsReport(metricReport)

		err := metricReport.AddMetrics(metricsArr[idxRange.Low:idxRange.High]...)

		if err != nil {
			return filenames, err
		}

		metadata.UpdateMetricsReport(metricReport)

		marshallBytes, err := json.Marshal(metricReport)
		logger.Debug(string(marshallBytes))
		if err != nil {
			logger.Error(err, "failed to marshal metrics report", "report", metricReport)
			return nil, err
		}
		filename := filepath.Join(
			r.OutputDirectory,
			fmt.Sprintf("%s.json", metricReport.ReportSliceID.String()))

		err = ioutil.WriteFile(
			filename,
			marshallBytes,
			0600)

		if err != nil {
			logger.Error(err, "failed to write file", "file", filename)
			return nil, errors.Wrap(err, "failed to write file")
		}

		filenames = append(filenames, filename)
	}

	marshallBytes, err := json.Marshal(metadata)
	if err != nil {
		logger.Error(err, "failed to marshal report metadata", "metadata", metadata)
		return nil, err
	}

	filename := filepath.Join(r.OutputDirectory, "metadata.json")
	err = ioutil.WriteFile(filename, marshallBytes, 0600)
	if err != nil {
		logger.Error(err, "failed to write file", "file", filename)
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
