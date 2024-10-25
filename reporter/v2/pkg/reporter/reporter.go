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
	"strings"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/google/uuid"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter/schema/common"
	marketplacecommon "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/prometheus"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	additionalLabels = []model.LabelName{"pod", "namespace", "service", "persistentvolumeclaim"}
	logger           = logf.Log.WithName("reporter")
	mdefLabels       = []model.LabelName{"meter_group", "meter_kind", "metric_aggregation", "metric_label", "metric_query", "name", "namespace", "workload_type"}
)

const (
	ErrNoMeterDefinitionsFound = errors.Sentinel("no meterDefinitions found")
	WarningDuplicateData       = errors.Sentinel("duplicate data")
	WarningPrintTemplate       = errors.Sentinel("template error")
)

var warningsFilter = map[error]interface{}{
	WarningDuplicateData: nil,
	WarningPrintTemplate: nil,
}

// Goals of the reporter:
// Get current meters to query
//
// # Build a report for hourly since last reprot
//
// # Query all the meters
//
// # Break up reports into manageable chunks
//
// # Upload to insights
//
// Update the CR status for each report and queue
type MarketplaceReporter struct {
	PrometheusAPI
	MktConfig        *marketplacev1alpha1.MarketplaceConfig
	report           *marketplacev1alpha1.MeterReport
	meterDefinitions MeterDefinitionReferences
	k8sResources     []interface{}
	*Config
	schemaDataBuilder common.DataBuilder
	reportWriter      common.ReportWriter
}

type ReportName types.NamespacedName
type Namespace string

func NewMarketplaceReporter(
	config *Config,
	report *marketplacev1alpha1.MeterReport,
	MktConfig *marketplacev1alpha1.MarketplaceConfig,
	api *PrometheusAPI,
	meterDefinitions MeterDefinitionReferences,
	k8sResources []interface{},
	schemaDataBuilder common.DataBuilder,
	reportWriter common.ReportWriter,
) (*MarketplaceReporter, error) {
	return &MarketplaceReporter{
		PrometheusAPI:     *api,
		MktConfig:         MktConfig,
		report:            report,
		Config:            config,
		meterDefinitions:  meterDefinitions,
		k8sResources:      k8sResources,
		schemaDataBuilder: schemaDataBuilder,
		reportWriter:      reportWriter,
	}, nil
}

func (r *MarketplaceReporter) CollectMetrics(ctxIn context.Context) (map[string]common.SchemaMetricBuilder, []error, []error, error) {
	ctx, cancel := context.WithCancel(ctxIn)
	defer cancel()

	resultsMap := make(map[string]common.SchemaMetricBuilder)
	var resultsMapMutex sync.Mutex

	errorList := []error{}
	warningsList := []error{}

	// data channels ; closed by this func
	meterDefsChan := make(chan *meterDefPromQuery)
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

	go r.Query(
		ctx,
		r.report.Spec.StartTime.Time,
		r.report.Spec.EndTime.Time,
		meterDefsChan,
		promModelsChan,
		queryDone,
		errorsChan)

	logger.Info("starting processing")

	go r.Process(
		ctx,
		promModelsChan,
		resultsMap,
		&resultsMapMutex,
		processDone,
		errorsChan)

	occurred := map[string]interface{}{}

	// Collect errors function
	go func() {
		for {
			if err, more := <-errorsChan; more {
				if _, ok := warningsFilter[errors.Cause(err)]; ok {
					if _, ok := occurred[err.Error()]; !ok {
						warningsList = append(warningsList, err)
					}
				} else {
					if _, ok := occurred[err.Error()]; !ok {
						errorList = append(errorList, err)
						occurred[err.Error()] = nil
						logger.Error(err, "error occurred processing")
					}
				}
			} else {
				errorDone <- true
				return
			}
		}
	}()

	logger.Info("sending queries")

	err := r.ProduceMeterDefinitions(r.meterDefinitions, meterDefsChan)
	if err != nil {
		logger.Error(err, "error occurred processing")
		errorsChan <- err
	}

	logger.Info("sending queries done")
	close(meterDefsChan)

	<-queryDone
	logger.Info("querying done")
	close(promModelsChan)

	<-processDone
	logger.Info("processing done")
	close(errorsChan)

	<-errorDone

	return resultsMap, errorList, warningsList, errors.Combine(errorList...)
}

type meterDefPromModel struct {
	mdef *meterDefPromQuery
	model.Value
	MetricName string
	Type       marketplacecommon.WorkloadType
}

type meterDefPromQuery struct {
	uid           string
	query         *PromQuery
	meterDefLabel *marketplacecommon.MeterDefPrometheusLabels
	meterGroup    string
	meterKind     string
	label         string
}

func (s *meterDefPromQuery) String() string {
	return "[" + s.uid + " " + s.meterGroup + " " + s.meterKind + " " + s.label + "]"
}

func transformMeterDefinitionReference(
	ref v1beta1.MeterDefinitionReference,
	start, end time.Time,
) ([]*meterDefPromQuery, error) {
	slice := make([]*meterDefPromQuery, 0, 1)
	labels, err := ref.ToPrometheusLabels()

	if err != nil {
		logger.Error(err, "error encountered")
		return slice, err
	}

	for _, labelStruct := range labels {
		labelMap, err := labelStruct.ToLabels()

		if err != nil {
			logger.Error(err, "error encountered")
			return slice, err
		}

		promQuery, err := buildPromQuery(labelMap, start, end)

		if err != nil {
			logger.Error(err, "error encountered")
			return slice, err
		}

		slice = append(slice, promQuery)
	}

	return slice, nil
}

type MeterDefKey struct {
	Name, Namespace string
}

func (r *MarketplaceReporter) ProduceMeterDefinitions(
	meterDefinitions MeterDefinitionReferences,
	meterDefsChan chan *meterDefPromQuery,
) error {
	definitionSet := make(map[types.NamespacedName][]*meterDefPromQuery)
	start, end := r.report.Spec.StartTime.Time.UTC(), r.report.Spec.EndTime.Time.Add(-1*time.Second).UTC()

	var rhmAccountExists bool
	cond := r.MktConfig.Status.Conditions.GetCondition(marketplacev1alpha1.ConditionRHMAccountExists)
	if cond != nil && cond.IsTrue() {
		rhmAccountExists = true
	} else {
		rhmAccountExists = false
	}

	for _, ref := range meterDefinitions {
		queries, err := transformMeterDefinitionReference(ref, start, end)

		if err != nil {
			logger.Error(err, "error transforming meter definition references", "name", ref.Name, "namespace", ref.Namespace)
			return err
		}

		key := types.NamespacedName{Name: ref.Name, Namespace: ref.Namespace}
		if _, ok := definitionSet[key]; !ok {
			definitionSet[key] = queries
		}
	}

	savedSet, err := r.getMeterDefinitions()

	if err != nil {
		logger.Error(err, "error retrieving saved meter defs")
		return err
	}

	for key := range definitionSet {
		logger.Info("meter definitions provided", "key", key)
	}

	for key, val := range savedSet {
		if _, ok := definitionSet[key]; !ok {
			logger.V(4).Info("could not find meter def in set so adding", "name", key.Name, "namespace", key.Namespace)
			localKey, localVal := key, val
			definitionSet[localKey] = localVal
		}
	}

	for key, val := range definitionSet {
		logger.V(4).Info("sending", "key", key)
		for _, query := range val {
			// if RHM/Software Central account does not exist,
			// skip generating MeterReport for MeterDefinitions that are type license or billable
			if !rhmAccountExists &&
				(query.query.MetricType == marketplacecommon.MetricTypeEmpty || // metricType empty is treated as MetricTypeLincense
					query.query.MetricType == marketplacecommon.MetricTypeBillable ||
					query.query.MetricType == marketplacecommon.MetricTypeLicense) {
				continue
			}
			localQ := query
			logger.V(4).Info("sending q", "q", localQ)
			meterDefsChan <- localQ
		}
	}

	return nil
}

func (r *MarketplaceReporter) WriteReport(
	reportID uuid.UUID,
	metrics map[string]common.SchemaMetricBuilder,
) ([]string, error) {
	return r.reportWriter.WriteReport(reportID, metrics, r.Config.OutputDirectory, *r.Config.MetricsPerFile)
}

func (r *MarketplaceReporter) getMeterDefinitions() (map[types.NamespacedName][]*meterDefPromQuery, error) {
	var result model.Value
	var warnings v1.Warnings
	var err error

	err = utils.Retry(func() error {
		query := &MeterDefinitionQuery{
			Start: r.report.Spec.StartTime.Time.UTC(),
			End:   r.report.Spec.EndTime.Time.Add(-1 * time.Millisecond).UTC(),
			Step:  time.Hour,
		}

		q, _ := query.Print()
		logger.Info("output", "query", q)

		result, warnings, err = r.QueryMeterDefinitions(query)

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
		return nil, err
	}

	logger.V(4).Info("result", "result", result)

	switch result.Type() {
	case model.ValMatrix:
		results, errs := getQueries(result.(model.Matrix))

		if len(errs) != 0 {
			err := errors.Combine(errs...)
			logger.Error(err, "errors encountered")
			return nil, err
		}

		return results, nil
	default:
		err := errors.NewWithDetails("result type is unprocessable", "type", result.Type().String())
		logger.Error(err, "error encountered")
		return nil, err
	}
}

// Report namespace labels of format swc.saas.ibm.com/key=value
const (
	labelPrefix    = "label_"
	swcLabelPrefix = "label_swc_saas_ibm_com_"
)

func (r *MarketplaceReporter) getNamespaceLabels(namespaces ...string) (map[string]map[string]string, error) {
	var result model.Value
	var warnings v1.Warnings
	var err error
	nsLabels := make(map[string]map[string]string)

	err = utils.Retry(func() error {
		query := &NamespacesQuery{
			Start:      r.report.Spec.StartTime.Time.UTC(),
			End:        r.report.Spec.EndTime.Time.Add(-1 * time.Millisecond).UTC(),
			Step:       time.Hour,
			Namespaces: namespaces,
		}

		result, warnings, err = r.QueryNamespaceLabels(query)

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
		return nsLabels, err
	}

	logger.V(4).Info("result", "result", result)

	// Multiple matrixVals may exist for the same namespace for a query range, if k8s labels were added/removed
	// namespace=myns label_firstlabel=1 label_secondlabel=2
	// namespace=myns label_firstlabel=1 label_secondlabel=2 label_thirdlabel=3
	// aggregate the labels for the query range

	switch result.Type() {
	case model.ValMatrix:
		matrixVals := result.(model.Matrix)
		for _, matrix := range matrixVals {
			pLabels := getAllKeysFromMetric(matrix.Metric)
			kvMap, err := kvToMap(pLabels)
			if err != nil {
				return nsLabels, err
			}

			// range over prom label map and add k8s label_A_LABEL to namespace label map
			// add only label_swc_saas_ibm_com_ and trim the label_ prefix
			namespace, ok := kvMap["namespace"]
			if ok {
				k8sLabels := make(map[string]string)
				for k, v := range kvMap {
					if strings.HasPrefix(k, swcLabelPrefix) {
						k8sLabels[strings.TrimPrefix(k, labelPrefix)] = v.(string)
					}
				}
				nsLabels[namespace.(string)] = k8sLabels
			}
		}
		return nsLabels, nil
	default:
		err := errors.NewWithDetails("result type is unprocessable", "type", result.Type().String())
		logger.Error(err, "error encountered")
		return nsLabels, err
	}
}

func (r *MarketplaceReporter) Query(
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
			val, warnings, err = r.ReportQuery(query)

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

		logger.V(4).Info("val", "val", val)

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

func (r *MarketplaceReporter) Process(
	ctx context.Context,
	inPromModels <-chan meterDefPromModel,
	results map[string]common.SchemaMetricBuilder,
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
				logger.V(4).Info("adding metric", "pmodel", pmodel.mdef.meterDefLabel, "metric", matrix.Metric)

				for _, pair := range matrix.Values {
					func() {
						labels := getAllKeysFromMetric(matrix.Metric)
						kvMap, err := kvToMap(labels)
						name := ""
						namespace := ""

						if pmodel.mdef.query.MeterDef.Name != "" &&
							pmodel.mdef.query.MeterDef.Namespace != "" {
							name = pmodel.mdef.query.MeterDef.Name
							namespace = pmodel.mdef.query.MeterDef.Namespace
							kvMap["meter_def_name"] = name
							kvMap["meter_def_namespace"] = namespace
						}

						if err != nil {
							logger.Error(err, "failed to get kvmap")
							errorsch <- errors.Wrap(err, "failed to get kvmap")
							return
						}

						meterDef := pmodel.mdef.meterDefLabel
						record, err := meterDef.PrintTemplate(&marketplacecommon.ReportLabels{
							Label: kvMap,
						}, pair)

						if err != nil {
							err = errors.WithMessagef(WarningPrintTemplate, "%s/%s %s", namespace, name, err.Error())
							errorsch <- err
							return
						}

						// Get the Labels for the namespace from query kube_namespace_labels{}
						// A query grouped by namespace will have a resource namespace
						// Otherwise collect the labels of the meterdef namespace
						ns := ""
						if record.ResourceNamespace != "" {
							ns = record.ResourceNamespace
						} else if record.ExportedMeterDefNamespace != "" {
							ns = record.ExportedMeterDefNamespace
						}
						if ns != "" {
							record.NamespaceLabels, err = r.getNamespaceLabels(ns)
							if err != nil {
								logger.Error(err, "failed to get namespace labels")
								errorsch <- errors.Wrap(err, "failed to get namespace labels")
								return
							}
						}

						// Kubernetes Infrastructure Resources
						record.K8sResources = r.k8sResources

						mutex.Lock()
						defer mutex.Unlock()

						dataBuilder, ok := results[record.Hash()]

						if !ok {
							dataBuilder = r.schemaDataBuilder.New()
							dataBuilder.SetClusterID(r.MktConfig.Spec.ClusterUUID)
							dataBuilder.SetAccountID(r.MktConfig.Spec.RhmAccountID)
							dataBuilder.SetReportInterval(
								common.Time(r.report.Spec.StartTime.Time),
								common.Time(r.report.Spec.EndTime.Time))
							results[record.Hash()] = dataBuilder
						}
						dataBuilder.AddMeterDefinitionLabels(record)
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

func buildPromQuery(labels interface{}, start, end time.Time) (*meterDefPromQuery, error) {
	meterDefLabels := &marketplacecommon.MeterDefPrometheusLabels{}
	err := meterDefLabels.FromLabels(labels)

	if err != nil {
		logger.Error(err, "failed to unmarshal labels")
		return nil, err
	}

	query := NewPromQueryFromLabels(meterDefLabels, start, end)

	if err != nil {
		logger.Error(err, "failed to create a template")
		return nil, err
	}

	return &meterDefPromQuery{
		uid:           meterDefLabels.UID,
		query:         query,
		meterDefLabel: meterDefLabels,
		meterGroup:    meterDefLabels.MeterGroup,
		meterKind:     meterDefLabels.MeterKind,
		label:         meterDefLabels.Metric,
	}, nil
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

func kvToMap(keysAndValues []interface{}) (map[string]interface{}, error) {
	metrics := make(map[string]interface{})
	if len(keysAndValues)%2 != 0 {
		return nil, errors.New("keyAndValues must be a length of 2")
	}

	chunks := utils.ChunkBy(keysAndValues, 2)

	for _, chunk := range chunks {
		if len(chunk) == 0 {
			continue
		}
		key := chunk[0]
		value := chunk[1]

		keyStr, ok := key.(string)

		if !ok {
			return nil, errors.Errorf("key type %t is not a string", key)
		}

		metrics[keyStr] = value
	}

	return metrics, nil
}
