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
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/version"
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

	go r.retrieveMeterDefinitions(
		meterDefsChan,
		errorsChan,
		meterDefsDone)

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
	uid        string
	query      *PromQuery
	meterGroup string
	meterKind  string
	label      string
}

func (s *meterDefPromQuery) String() string {
	return "[" + s.uid + " " + s.meterGroup + " " + s.meterKind + " " + s.label + "]"
}

func (r *MarketplaceReporter) retrieveMeterDefinitions(
	meterDefsChan chan *meterDefPromQuery,
	errorChan chan error,
	meterDefsDone chan bool,
) {
	var result model.Value
	var warnings v1.Warnings
	var err error

	defer func() {
		meterDefsDone <- true
	}()

	err = utils.Retry(func() error {
		query := &MeterDefinitionQuery{
			Start: r.report.Spec.StartTime.Time,
			End:   r.report.Spec.EndTime.Time,
			Step:  time.Hour,
		}

		q, _ := query.Print()
		logger.Info("output", "query", q)

		result, warnings, err = r.queryMeterDefinitions(query)

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
		errorChan <- err
		return
	}

	// Use defined on the report queries for fallback for a while
	definedPromQueries := []*meterDefPromQuery{}

	for _, mdef := range r.meterDefinitions {
		labels := mdef.ToPrometheusLabels()
		for _, label := range labels {
			promQuery := buildPromQuery(label, r.report.Spec.StartTime.Time, r.report.Spec.EndTime.Time)
			logger.Info("found prom queries on the report", "uidQuery", promQuery)
			definedPromQueries = append(definedPromQueries, promQuery)
		}
	}

	switch result.Type() {
	case model.ValMatrix:
		matrixVals := result.(model.Matrix)
		for _, matrix := range matrixVals {
			meterGroup, _ := getMatrixValue(matrix.Metric, "meter_group")
			meterKind, _ := getMatrixValue(matrix.Metric, "meter_kind")

			// skip 0 data groups
			if meterGroup == "" && meterKind == "" {
				continue
			}

			var min, max time.Time
			for i, v := range matrix.Values {
				if i == 0 {
					min = v.Timestamp.Time()
					max = v.Timestamp.Time()
				}

				if v.Timestamp.Time().Before(min) {
					min = v.Timestamp.Time()
				}
				if v.Timestamp.Time().After(max) {
					max = v.Timestamp.Time()
				}
			}

			max = max.Add(-time.Second)
			promQuery := buildPromQuery(matrix.Metric, min, max)

			logger.Info("getting query", "query", promQuery.String())
			meterDefsChan <- promQuery
		}
	case model.ValString:
	case model.ValVector:
	case model.ValScalar:
	case model.ValNone:
	}

	return
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

		outPromModels <- meterDefPromModel{
			mdef:       mdef,
			Value:      val,
			MetricName: query.Metric,
			Type:       query.Type,
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
	done chan bool,
	errorsch chan error,
) {
	syncProcess := func(
		pmodel meterDefPromModel,
		name string,
		m model.Value,
	) {
		//# do the work
		switch m.Type() {
		case model.ValMatrix:
			matrixVals := m.(model.Matrix)

			for _, matrix := range matrixVals {
				logger.V(4).Info("adding metric", "metric", matrix.Metric)

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
							errorsch <- errors.New("can't find objName")
							return
						}

						key := MetricKey{
							ReportPeriodStart: r.report.Spec.StartTime.Format(time.RFC3339),
							ReportPeriodEnd:   r.report.Spec.EndTime.Format(time.RFC3339),
							IntervalStart:     pair.Timestamp.Time().Format(time.RFC3339),
							IntervalEnd:       pair.Timestamp.Add(time.Hour).Time().Format(time.RFC3339),
							MeterDomain:       pmodel.mdef.meterGroup,
							MeterKind:         pmodel.mdef.meterKind,
							Namespace:         namespace,
							ResourceName:      objName,
							Label:             pmodel.mdef.label,
						}

						key.Init(r.mktconfig.Spec.ClusterUUID)

						mutex.Lock()
						defer mutex.Unlock()

						base, ok := results[key]

						if !ok {
							base = &MetricBase{
								Key: key,
							}
						}

						logger.V(4).Info("adding pair", "metric", matrix.Metric, "pair", pair)
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
			syncProcess(pmodel, pmodel.MetricName, pmodel.Value)
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

		if r.Config.UploaderTarget != UploaderTargetRedHatInsights {
			metricReport.AddMetadata(metadata.ToFlat())
		}

		metadata.AddMetricsReport(metricReport)

		err := metricReport.AddMetrics(metricsArr[idxRange.Low:idxRange.High]...)

		if err != nil {
			return filenames, err
		}

		metadata.UpdateMetricsReport(metricReport)

		marshallBytes, err := json.Marshal(metricReport)
		logger.V(4).Info(string(marshallBytes))
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

func getMatrixValue(m interface{}, labelName string) (string, bool) {
	switch iMap := m.(type) {
	case model.Metric:
		if v, ok := iMap[model.LabelName(labelName)]; ok {
			return string(v), true
		}
		return "", false
	case map[string]string:

		if v, ok := iMap[labelName]; ok {
			return string(v), true
		}
		return "", false
	default:
		return "", false
	}
}

func buildPromQuery(labels interface{}, start, end time.Time) *meterDefPromQuery {
	meter_definition_uid, _ := getMatrixValue(labels, "meter_definition_uid")
	metricLabel, _ := getMatrixValue(labels, "metric_label")
	workloadType, _ := getMatrixValue(labels, "workload_type")
	name, _ := getMatrixValue(labels, "name")
	namespace, _ := getMatrixValue(labels, "namespace")
	metricAggregation, _ := getMatrixValue(labels, "metric_aggregation")
	metricQuery, _ := getMatrixValue(labels, "metric_query")
	meterGroup, _ := getMatrixValue(labels, "meter_group")
	meterKind, _ := getMatrixValue(labels, "meter_kind")

	if metricAggregation == "" {
		metricAggregation = "sum"
	}

	query := NewPromQuery(&PromQueryArgs{
		Metric: metricLabel,
		Type:   marketplacev1alpha1.WorkloadType(workloadType),
		MeterDef: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
		Query:         metricQuery,
		Start:         start,
		End:           end,
		Step:          time.Hour,
		AggregateFunc: metricAggregation,
	})

	return &meterDefPromQuery{uid: meter_definition_uid, query: query, meterGroup: meterGroup, meterKind: meterKind, label: metricLabel}
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
