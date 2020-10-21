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

package reporter

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/google/uuid"
	"github.com/meirf/gopart"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	additionalLabels = []model.LabelName{"pod", "namespace", "service", "persistentvolumeclaim"}
	logger           = logf.Log.WithName("reporter")
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
	k8sclient         client.Client
	mktconfig         *marketplacev1alpha1.MarketplaceConfig
	report            *marketplacev1alpha1.MeterReport
	meterDefinitions  []marketplacev1alpha1.MeterDefinition
	prometheusService *corev1.Service
	*Config
}

type ReportName types.NamespacedName

func NewMarketplaceReporter(
	config *Config,
	k8sclient client.Client,
	report *marketplacev1alpha1.MeterReport,
	mktconfig *marketplacev1alpha1.MarketplaceConfig,
	meterDefinitions []marketplacev1alpha1.MeterDefinition,
	prometheusService *corev1.Service,
	apiClient api.Client,
) (*MarketplaceReporter, error) {
	return &MarketplaceReporter{
		api:               v1.NewAPI(apiClient),
		k8sclient:         k8sclient,
		mktconfig:         mktconfig,
		report:            report,
		meterDefinitions:  meterDefinitions,
		Config:            config,
		prometheusService: prometheusService,
	}, nil
}

var ErrNoMeterDefinitionsFound = errors.New("no meterDefinitions found")

func (r *MarketplaceReporter) CollectMetrics(ctxIn context.Context) (map[MetricKey]*MetricBase, []error, error) {
	ctx, cancel := context.WithCancel(ctxIn)
	defer cancel()

	resultsMap := make(map[MetricKey]*MetricBase)
	var resultsMapMutex sync.Mutex

	if len(r.meterDefinitions) == 0 {
		logger.Info("no meterdefs found")
		return resultsMap, []error{}, nil
	}

	// data channels ; closed by this func
	meterDefsChan := make(chan *marketplacev1alpha1.MeterDefinition, len(r.meterDefinitions))
	promModelsChan := make(chan meterDefPromModel)

	// error channels
	errorsChan := make(chan error)

	// done channels
	queryDone := make(chan bool)
	processDone := make(chan bool)
	errorDone := make(chan bool)

	defer close(queryDone)
	defer close(processDone)
	defer close(errorDone)

	logger.Info("starting query")

	go r.query(
		ctx,
		r.report.Spec.StartTime.Time,
		r.report.Spec.EndTime.Time,
		meterDefsChan,
		promModelsChan,
		queryDone,
		errorsChan)

	logger.Info("starting processing")

	go r.process(
		ctx,
		promModelsChan,
		resultsMap,
		&resultsMapMutex,
		r.report,
		processDone,
		errorsChan)

	// send & close data pipe
	for _, meterDef := range r.meterDefinitions {
		meterDefsChan <- &meterDef
	}
	close(meterDefsChan)

	errorList := []error{}

	// Collect errors function
	go func() {
		for {
			if err, more := <-errorsChan; more {
				logger.Error(err, "error occurred processing")
				errorList = append(errorList, err)
			} else {
				errorDone <- true
				return
			}
		}
	}()

	<-queryDone
	logger.Info("querying done")
	close(promModelsChan)

	<-processDone
	logger.Info("processing done")
	close(errorsChan)

	<-errorDone

	return resultsMap, errorList, errors.Combine(errorList...)
}

type meterDefPromModel struct {
	*marketplacev1alpha1.MeterDefinition
	additionalFieldMap map[string]string
	model.Value
	MetricName string
	Type       v1alpha1.WorkloadType
}

// builds the queries that get sent to prometheus
func (r *MarketplaceReporter) query(
	ctx context.Context,
	startTime, endTime time.Time,
	inMeterDefs <-chan *marketplacev1alpha1.MeterDefinition,
	outPromModels chan<- meterDefPromModel,
	done chan bool,
	errorsch chan<- error,
) {
	queryProcess := func(mdef *marketplacev1alpha1.MeterDefinition) {
		for _, workload := range mdef.Spec.Workloads {
			for _, metric := range workload.MetricLabels {
				logger.Info("query", "metric", metric)
				// TODO: use metadata to build a smart roll up
				// Guage = delta
				// Counter = increase
				// Histogram and summary are unsupported
				var errorArr []error
				additionalFieldMap := make(map[string]string)

				query := &PromQuery{
					Metric: metric.Label,
					Type:   workload.WorkloadType,
					MeterDef: types.NamespacedName{
						Name:      mdef.Name,
						Namespace: mdef.Namespace,
					},
					Query:         metric.Query,
					Time:          "60m",
					Start:         startTime,
					End:           endTime,
					Step:          time.Hour,
					AggregateFunc: metric.Aggregation,
				}
				logger.Info("output", "query", query.String())

				if metric.AdditionalFields != nil {
					queryValidationErrors := processAdditionalFields(metric.Label, metric.Query, metric.AdditionalFields, additionalFieldMap)
					for _, err := range queryValidationErrors {
						errorArr = append(errorArr, err)
					}
				}

				var val model.Value
				var warnings v1.Warnings

				err := utils.Retry(func() error {
					var err error
					val, warnings, err = r.queryRange(query)

					if err != nil {
						return errors.Wrap(err, "error with query")
					}

					return nil
				}, *r.Retry)

				if err != nil {
					errorArr = append(errorArr, err)
				}

				if warnings != nil {
					logger.Info("warnings %v", warnings)
				}

				if len(errorArr) > 0 {
					for _, err := range errorArr {
						logger.Error(err, "error encountered")
						errorsch <- err
					}

					return
				}

				outPromModels <- meterDefPromModel{mdef, additionalFieldMap, val, metric.Label, query.Type}
			}
		}
	}

	wgWait(ctx, "queryProcess", *r.MaxRoutines, done, func() {
		for mdef := range inMeterDefs {
			queryProcess(mdef)
		}
	})
}

func (r *MarketplaceReporter) process(
	ctx context.Context,
	inPromModels <-chan meterDefPromModel,
	results map[MetricKey]*MetricBase,
	mutex sync.Locker,
	report *marketplacev1alpha1.MeterReport,
	done chan bool,
	errorsch chan error,
) {
	syncProcess := func(
		pmodel meterDefPromModel,
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
				logger.Info("adding metric", "metric", matrix.Metric)

				for _, pair := range matrix.Values {
					func() {

						labels := getKeysFromMetric(matrix.Metric, additionalLabels)
						labelMatrix, err := kvToMap(labels)

						if err != nil {
							errorsch <- errors.Wrap(err, "failed adding additional labels")
							return
						}

						var objName string
						namespace := labelMatrix["namespace"].(string)

						switch pmodel.Type {
						case v1alpha1.WorkloadTypePVC:
							if pvc, ok := labelMatrix["persistentvolumeclaim"]; ok {
								objName = pvc.(string)
							}
						case v1alpha1.WorkloadTypePod:
							if pod, ok := labelMatrix["pod"]; ok {
								objName = pod.(string)
							}
						case v1alpha1.WorkloadTypeServiceMonitor:
							fallthrough
						case v1alpha1.WorkloadTypeService:
							if service, ok := labelMatrix["service"]; ok {
								objName = service.(string)
							}
						}

						if objName == "" {
							errorsch <- errors.New("can't find objName")
							return
						}

						key := MetricKey{
							ReportPeriodStart: report.Spec.StartTime.Format(time.RFC3339),
							ReportPeriodEnd:   report.Spec.EndTime.Format(time.RFC3339),
							IntervalStart:     pair.Timestamp.Time().Format(time.RFC3339),
							IntervalEnd:       pair.Timestamp.Add(time.Hour).Time().Format(time.RFC3339),
							MeterDomain:       mdef.Spec.Group,
							MeterKind:         mdef.Spec.Kind,
						}

						key.Init(r.mktconfig.Spec.ClusterUUID, objName, namespace)

						mutex.Lock()
						defer mutex.Unlock()

						base, ok := results[key]

						if !ok {
							base = &MetricBase{
								Key: key,
							}
						}

						logger.Info("adding pair", "metric", matrix.Metric, "pair", pair)
						metricPairs := []interface{}{name, pair.Value.String()}

						for key, value := range pmodel.additionalFieldMap {
							labels = append(labels, key, value)
						}

						err = base.AddAdditionalLabels(labels...)

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

	wgWait(ctx, "syncProcess", *r.MaxRoutines, done, func() {
		for pmodel := range inPromModels {
			syncProcess(pmodel, pmodel.MetricName, pmodel.MeterDefinition, report, pmodel.Value)
		}
	})
}

func (r *MarketplaceReporter) WriteReport(
	source uuid.UUID,
	metrics map[MetricKey]*MetricBase) ([]string, error) {
	metadata := NewReportMetadata(source, ReportSourceMetadata{
		RhmAccountID: r.mktconfig.Spec.RhmAccountID,
		RhmClusterID: r.mktconfig.Spec.ClusterUUID,
	})

	var partitionSize = *r.MetricsPerFile

	metricsArr := make([]*MetricBase, 0, len(metrics))

	filedir := filepath.Join(r.Config.OutputDirectory, source.String())
	err := os.Mkdir(filedir, 0755)

	if err != nil {
		return []string{}, errors.Wrap(err, "error creating directory")
	}

	for _, v := range metrics {
		metricsArr = append(metricsArr, v)
	}

	filenames := []string{}

	for idxRange := range gopart.Partition(len(metricsArr), partitionSize) {
		metricReport := NewReport()
		metadata.AddMetricsReport(metricReport)

		err := metricReport.AddMetricsToReport(metricsArr[idxRange.Low:idxRange.High]...)

		if err != nil {
			return filenames, err
		}

		metadata.UpdateMetricsReport(metricReport)

		marshallBytes, err := json.Marshal(metricReport)
		logger.Info(string(marshallBytes))
		if err != nil {
			logger.Error(err, "failed to marshal metrics report", "report", metricReport)
			return nil, err
		}
		filename := filepath.Join(
			filedir,
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

	filename := filepath.Join(filedir, "metadata.json")
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

func wgWait(ctx context.Context, processName string, maxRoutines int, done chan bool, waitFunc func()) {
	var wg sync.WaitGroup
	for w := 1; w <= maxRoutines; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			waitFunc()
		}()
	}

	wait := make(chan bool)
	defer close(wait)

	go func() {
		wg.Wait()
		wait <- true
	}()

	select {
	case <-ctx.Done():
		logger.Info("canceling wg", "name", processName)
	case <-wait:
		logger.Info("wg is done", "name", processName)
	}

	done <- true
}

func getLabelsFromMetricQuery(queryLabel string, originalQueryString string) (parsedLabels string) {
	arr := strings.Split(originalQueryString, "(")
	for _, word := range arr {
		if strings.Contains(word, queryLabel) {
			parsedLabels = utils.GetStringInBetween(originalQueryString, "{", "}")
		}
	}

	return parsedLabels
}

func processAdditionalFields(queryLabel string, originalQueryString string, additionalFieldsFromMeterDef []string, additionalFieldMap map[string]string) []error {
	var queryValidationErrors []error
	parsedQueryString := getLabelsFromMetricQuery(queryLabel, originalQueryString)
	labelPairs := strings.Split(parsedQueryString, ",")

	for _, additionalField := range additionalFieldsFromMeterDef {
		if !strings.Contains(parsedQueryString, additionalField) {
			msg := fmt.Sprintf("Query doesn't contain a label key for additionalField: %s", additionalField)
			queryValidationErrors = append(queryValidationErrors, errors.New(msg))
		} else {
			for _, keyValuePairString := range labelPairs {
				keyValuePairArray := strings.Split(keyValuePairString, "=")
				if additionalField == keyValuePairArray[0] {
					additionalFieldMap[keyValuePairArray[0]] = keyValuePairArray[1]
				}
			}
		}
	}

	return queryValidationErrors
}
