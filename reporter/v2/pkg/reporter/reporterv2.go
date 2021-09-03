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
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter/schema/common"
	schemav2alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter/schema/v2alpha1"
	marketplacecommon "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/prometheus"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/version"
	"k8s.io/apimachinery/pkg/types"
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

type MarketplaceReporterV2 struct {
	PrometheusAPI
	mktconfig        *marketplacev1alpha1.MarketplaceConfig
	report           *marketplacev1alpha1.MeterReport
	meterDefinitions MeterDefinitionReferences
	*Config
}

func NewMarketplaceReporterV2(
	config *Config,
	report *marketplacev1alpha1.MeterReport,
	mktconfig *marketplacev1alpha1.MarketplaceConfig,
	api *PrometheusAPI,
	meterDefinitions MeterDefinitionReferences,
) (*MarketplaceReporterV2, error) {
	return &MarketplaceReporterV2{
		PrometheusAPI:    *api,
		mktconfig:        mktconfig,
		report:           report,
		Config:           config,
		meterDefinitions: meterDefinitions,
	}, nil
}

func (r *MarketplaceReporterV2) CollectMetrics(ctxIn context.Context) (map[string]*schemav2alpha1.MarketplaceReportDataBuilder, []error, []error, error) {
	ctx, cancel := context.WithCancel(ctxIn)
	defer cancel()

	resultsMap := make(map[string]*schemav2alpha1.MarketplaceReportDataBuilder)
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

	r.ProduceMeterDefinitions(
		r.meterDefinitions,
		meterDefsChan)

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

func (r *MarketplaceReporterV2) ProduceMeterDefinitions(
	meterDefinitions MeterDefinitionReferences,
	meterDefsChan chan *meterDefPromQuery,
) error {
	definitionSet := make(map[types.NamespacedName][]*meterDefPromQuery)
	start, end := r.report.Spec.StartTime.Time.UTC(), r.report.Spec.EndTime.Time.Add(-1*time.Second).UTC()

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

	savedSet, err := r.getBackedUpMeterDefinitions()

	if err != nil {
		logger.Error(err, "error retrieving saved meter defs")
		return err
	}

	for key, _ := range definitionSet {
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
			localQ := query
			logger.V(4).Info("sending q", "q", localQ)
			meterDefsChan <- localQ
		}
	}

	return nil
}

func (r *MarketplaceReporterV2) getBackedUpMeterDefinitions() (map[types.NamespacedName][]*meterDefPromQuery, error) {
	var result model.Value
	var warnings v1.Warnings
	var err error

	err = utils.Retry(func() error {
		query := &MeterDefinitionQuery{
			Start: r.report.Spec.StartTime.Time.UTC(),
			End:   r.report.Spec.EndTime.Time.Add(-1 * time.Second).UTC(),
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

func (r *MarketplaceReporterV2) Query(
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

func (r *MarketplaceReporterV2) Process(
	ctx context.Context,
	inPromModels <-chan meterDefPromModel,
	results map[string]*schemav2alpha1.MarketplaceReportDataBuilder,
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

						if pmodel.mdef.query.MeterDef.Name != "" &&
							pmodel.mdef.query.MeterDef.Namespace != "" {
							kvMap["meter_def_name"] = pmodel.mdef.query.MeterDef.Name
							kvMap["meter_def_namespace"] = pmodel.mdef.query.MeterDef.Namespace
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
							errorsch <- err
							return
						}

						mutex.Lock()
						defer mutex.Unlock()

						dataBuilder, ok := results[record.Hash()]

						if !ok {
							dataBuilder = (&schemav2alpha1.MarketplaceReportDataBuilder{}).
								SetClusterID(r.mktconfig.Spec.ClusterUUID).
								SetAccountID(r.mktconfig.Spec.RhmAccountID).
								SetReportInterval(
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

func (r *MarketplaceReporterV2) WriteReport(
	source uuid.UUID,
	metrics map[string]*schemav2alpha1.MarketplaceReportDataBuilder,
) ([]string, error) {

	env := common.ReportProductionEnv
	envAnnotation, ok := r.mktconfig.Annotations["marketplace.redhat.com/environment"]

	if ok && envAnnotation == common.ReportSandboxEnv.String() {
		env = common.ReportSandboxEnv
	}

	metadata := schemav2alpha1.SourceMetadata{
		RhmAccountID:   r.mktconfig.Spec.RhmAccountID,
		RhmClusterID:   r.mktconfig.Spec.ClusterUUID,
		RhmEnvironment: env,
		Version:        version.Version,
		ReportVersion:  schemav2alpha1.Version,
	}

	var partitionSize = *r.MetricsPerFile

	metricsArr := make([]*schemav2alpha1.MarketplaceReportDataBuilder, 0, len(metrics))

	filedir := filepath.Join(r.Config.OutputDirectory, source.String())
	err := os.Mkdir(filedir, 0755)

	if err != nil && !errors.Is(err, os.ErrExist) {
		return []string{}, errors.Wrap(err, "error creating directory")
	}

	for _, v := range metrics {
		metricsArr = append(metricsArr, v)
	}

	filenames := []string{}
	reportErrors := []error{}

	for idxRange := range gopart.Partition(len(metricsArr), partitionSize) {
		metricReport := &schemav2alpha1.MarketplaceReportSlice{}
		reportSliceID := common.ReportSliceKey(uuid.New())

		if r.Config.UploaderTarget != UploaderTargetRedHatInsights {
			metricReport.Metadata = &metadata
		}

		for _, builder := range metricsArr[idxRange.Low:idxRange.High] {
			metric, err := builder.Build()

			if err != nil {
				reportErrors = append(reportErrors, err)
			}

			metricReport.Metrics = append(metricReport.Metrics, metric)
		}

		marshallBytes, err := json.Marshal(metricReport)
		logger.V(4).Info(string(marshallBytes))
		if err != nil {
			logger.Error(err, "failed to marshal metrics report", "report", metricReport)
			return nil, err
		}
		filename := filepath.Join(
			filedir,
			fmt.Sprintf("%s.json", reportSliceID.String()))

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

	manifest := &schemav2alpha1.Manifest{
		Type:    schemav2alpha1.AccountMetrics,
		Version: "1",
	}

	marshallBytes, err := json.Marshal(manifest)
	if err != nil {
		logger.Error(err, "failed to marshal report manifest", "manifest", manifest)
		return nil, err
	}

	filename := filepath.Join(filedir, "manifest.json")

	err = ioutil.WriteFile(filename, marshallBytes, 0600)
	if err != nil {
		logger.Error(err, "failed to write file", "file", filename)
		return nil, err
	}

	filenames = append(filenames, filename)

	return filenames, errors.Combine(reportErrors...)
}
