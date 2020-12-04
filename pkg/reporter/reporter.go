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
	"github.com/redhat-marketplace/redhat-marketplace-operator/version"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	additionalLabels = []model.LabelName{"pod", "namespace", "service", "persistentvolumeclaim"}
	logger           = logf.Log.WithName("reporter")
	mdefLabels       = []model.LabelName{"meter_group", "meter_kind", "metric_aggregation", "metric_label", "metric_query", "name", "namespace", "workload_type"}
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

	// Returns a set of elements without duplicates
	// Ignore labels such that a pod restart, meterdefinition recreate, or other labels do not generate a new unique element
	mdefQuery := "(meterdef_metric_label_info{} + ignoring(container, endpoint, instance, job, meter_definition_uid, pod, service) meterdef_metric_label_info{})"
	var result model.Value
	var warnings v1.Warnings
	var err error

	logger.Info("output", "query", mdefQuery)

	err = utils.Retry(func() error {
		qctx, qcancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer qcancel()

		timeRange := v1.Range{
			Start: r.report.Spec.StartTime.Time,
			End:   r.report.Spec.EndTime.Time,
			Step:  time.Hour,
		}

		result, warnings, err = r.api.QueryRange(qctx, mdefQuery, timeRange)

		if err != nil {
			logger.Error(err, "querying prometheus", "warnings", warnings)
			return err
		}
		if len(warnings) > 0 {
			logger.Info("warnings", "warnings", warnings)
		}

		return nil
	}, *r.Retry)

	if err != nil {
		logger.Error(err, "error encountered")
		return resultsMap, []error{}, err
	}

	errorList := []error{}

	// data channels ; closed by this func
	meterDefsChan := make(chan *meterDefPromQuery)
	promModelsChan := make(chan meterDefPromModel)

	// error channels
	errorsChan := make(chan error)

	// done channels
	meterDefsDone := make(chan bool)
	queryDone := make(chan bool)
	processDone := make(chan bool)
	errorDone := make(chan bool)

	defer close(meterDefsDone)
	defer close(queryDone)
	defer close(processDone)
	defer close(errorDone)

	logger.Info("starting build queries")

	// build queries func
	go func() {
		switch result.Type() {
		case model.ValMatrix:
			matrixVals := result.(model.Matrix)

			for _, matrix := range matrixVals {
				metricLabel, _ := getMatrixValue(matrix.Metric, "metric_label")
				workloadType, _ := getMatrixValue(matrix.Metric, "workload_type")
				name, _ := getMatrixValue(matrix.Metric, "name")
				namespace, _ := getMatrixValue(matrix.Metric, "namespace")
				metricAggregation, _ := getMatrixValue(matrix.Metric, "metric_aggregation")
				metricQuery, _ := getMatrixValue(matrix.Metric, "metric_query")
				meterGroup, _ := getMatrixValue(matrix.Metric, "meter_group")
				meterKind, _ := getMatrixValue(matrix.Metric, "meter_kind")

				query := &PromQuery{
					Metric: metricLabel,
					Type:   marketplacev1alpha1.WorkloadType(workloadType),
					MeterDef: types.NamespacedName{
						Name:      name,
						Namespace: namespace,
					},
					Query:         metricQuery,
					Time:          "60m",
					Start:         r.report.Spec.StartTime.Time,
					End:           r.report.Spec.EndTime.Time,
					Step:          time.Hour,
					AggregateFunc: metricAggregation,
				}

				meterDefsChan <- &meterDefPromQuery{query: query, meterGroup: meterGroup, meterKind: meterKind}
			}
		case model.ValString:
		case model.ValVector:
		case model.ValScalar:
		case model.ValNone:
			errorList = append(errorList, errors.Errorf("can't process model type=%s", result.Type()))
			return
		}

		meterDefsDone <- true
		return
	}()

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

	<-meterDefsDone
	logger.Info("build queries done")
	close(meterDefsChan)

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
	mdef *meterDefPromQuery
	model.Value
	MetricName string
	Type       v1alpha1.WorkloadType
}

type meterDefPromQuery struct {
	query      *PromQuery
	meterGroup string
	meterKind  string
}

func (r *MarketplaceReporter) query(
	ctx context.Context,
	startTime, endTime time.Time,
	inMeterDefs <-chan *meterDefPromQuery,
	outPromModels chan<- meterDefPromModel,
	done chan bool,
	errorsch chan<- error,
) {
	queryProcess := func(mdef *meterDefPromQuery) {
		// TODO: use metadata to build a smart roll up
		// Guage = delta
		// Counter = increase
		// Histogram and summary are unsupported
		query := mdef.query

		logger.Info("output", "query", query.String())

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

		if warnings != nil {
			logger.Info("warnings %v", warnings)
		}

		if err != nil {
			logger.Error(err, "error encountered")
			errorsch <- err
			return
		}

		outPromModels <- meterDefPromModel{mdef, val, query.Metric, query.Type}

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

						labels := getAllKeysFromMetric(matrix.Metric)

						var objName string

						namespace, _ := getMatrixValue(matrix.Metric, "namespace")

						switch pmodel.Type {
						case v1alpha1.WorkloadTypePVC:
							objName, _ = getMatrixValue(matrix.Metric, "persistentvolumeclaim")
						case v1alpha1.WorkloadTypePod:
							objName, _ = getMatrixValue(matrix.Metric, "pod")
						case v1alpha1.WorkloadTypeServiceMonitor:
							fallthrough
						case v1alpha1.WorkloadTypeService:
							objName, _ = getMatrixValue(matrix.Metric, "service")
						}

						if objName == "" {
							errorsch <- errors.New("can't fine objName")
							return
						}

						key := MetricKey{
							ReportPeriodStart: report.Spec.StartTime.Format(time.RFC3339),
							ReportPeriodEnd:   report.Spec.EndTime.Format(time.RFC3339),
							IntervalStart:     pair.Timestamp.Time().Format(time.RFC3339),
							IntervalEnd:       pair.Timestamp.Add(time.Hour).Time().Format(time.RFC3339),
							MeterDomain:       pmodel.mdef.meterGroup,
							MeterKind:         pmodel.mdef.meterKind,
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

						err := base.AddAdditionalLabels(labels...)

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
			syncProcess(pmodel, pmodel.MetricName, report, pmodel.Value)
		}
	})
}

func (r *MarketplaceReporter) WriteReport(
	source uuid.UUID,
	metrics map[MetricKey]*MetricBase) ([]string, error) {

	env := ReportProductionEnv
	envAnnotation, ok := r.mktconfig.Annotations["marketplace.redhat.com/environment"]

	if ok && envAnnotation == ReportSandboxEnv.String() {
		env = ReportSandboxEnv
	}

	metadata := NewReportMetadata(source, ReportSourceMetadata{
		RhmAccountID:   r.mktconfig.Spec.RhmAccountID,
		RhmClusterID:   r.mktconfig.Spec.ClusterUUID,
		RhmEnvironment: env,
		Version:        version.Version,
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

		err := metricReport.AddMetrics(metricsArr[idxRange.Low:idxRange.High]...)

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

func getAllKeysFromMetric(metric model.Metric) []interface{} {
	allLabels := make([]interface{}, 0, len(metric)*2)
	for k, v := range metric {
		allLabels = append(allLabels, string(k), string(v))
	}
	return allLabels
}

func getMatrixValue(m model.Metric, labelName string) (string, bool) {
	if v, ok := m[model.LabelName(labelName)]; ok {
		return string(v), true
	}
	return "", false
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
