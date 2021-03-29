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
	"encoding/json"
	"fmt"
	"time"

	"emperror.dev/errors"
	"github.com/cespare/xxhash"
	"github.com/google/uuid"
	"github.com/imdario/mergo"
	"github.com/mitchellh/mapstructure"
	"github.com/prometheus/common/model"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
)

type ReportEnvironment string

const (
	ReportProductionEnv ReportEnvironment = "production"
	ReportSandboxEnv    ReportEnvironment = "stage"
)

func (m ReportEnvironment) MarshalText() ([]byte, error) {
	return []byte(string(m)), nil
}

func (m *ReportEnvironment) UnmarshalText(data []byte) error {
	str := ReportEnvironment(string(data))
	*m = str
	return nil
}

func (m ReportEnvironment) String() string {
	return string(m)
}

type ReportMetadata struct {
	ReportID       uuid.UUID                            `json:"report_id"`
	Source         uuid.UUID                            `json:"source"`
	SourceMetadata ReportSourceMetadata                 `json:"source_metadata"`
	ReportSlices   map[ReportSliceKey]ReportSlicesValue `json:"report_slices"`
}

func (m *ReportMetadata) ToFlat() *ReportFlatMetadata {
	return &ReportFlatMetadata{
		ReportID: m.ReportID.String(),
		Source:   m.Source.String(),
		Metadata: m.SourceMetadata,
	}
}

type ReportSourceMetadata struct {
	RhmClusterID   string            `json:"rhmClusterId" mapstructure:"rhmClusterId"`
	RhmAccountID   string            `json:"rhmAccountId" mapstructure:"rhmAccountId"`
	RhmEnvironment ReportEnvironment `json:"rhmEnvironment,omitempty" mapstructure:"rhmEnvironment,omitempty"`
	Version        string            `json:"version,omitempty" mapstructure:"version,omitempty"`
}

type ReportFlatMetadata struct {
	ReportID string               `mapstructure:"report_id"`
	Source   string               `mapstructure:"source"`
	Metadata ReportSourceMetadata `mapstructure:",squash"`
}

func (d ReportFlatMetadata) MarshalJSON() ([]byte, error) {
	result := make(map[string]interface{})
	err := mapstructure.Decode(d, &result)
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(&result)
}

func (d *ReportFlatMetadata) UnmarshalJSON(data []byte) error {
	var jd map[string]interface{}
	if err := json.Unmarshal(data, &jd); err != nil {
		return err
	}
	if err := mapstructure.Decode(jd, d); err != nil {
		return err
	}
	return nil
}

type ReportSliceKey uuid.UUID

func (sliceKey ReportSliceKey) MarshalText() ([]byte, error) {
	return uuid.UUID(sliceKey).MarshalText()
}

func (sliceKey *ReportSliceKey) UnmarshalText(data []byte) error {
	id, err := uuid.NewUUID()

	if err != nil {
		return err
	}

	err = id.UnmarshalText(data)

	if err != nil {
		return err
	}

	*sliceKey = ReportSliceKey(id)
	return nil
}

func (sliceKey ReportSliceKey) String() string {
	return uuid.UUID(sliceKey).String()
}

type ReportSlicesValue struct {
	NumberMetrics int `json:"number_metrics"`
}

type MetricsReport struct {
	ReportSliceID ReportSliceKey           `json:"report_slice_id"`
	Metrics       []map[string]interface{} `json:"metrics"`
	Metadata      *ReportFlatMetadata      `json:"metadata,omitempty"`
}

type MetricKey struct {
	MetricID          string `mapstructure:"metric_id"`
	ReportPeriodStart string `mapstructure:"report_period_start"`
	ReportPeriodEnd   string `mapstructure:"report_period_end"`
	IntervalStart     string `mapstructure:"interval_start"`
	IntervalEnd       string `mapstructure:"interval_end"`
	MeterDomain       string `mapstructure:"domain"`
	MeterKind         string `mapstructure:"kind" template:""`
	MeterVersion      string `mapstructure:"version,omitempty"`
	Label             string `mapstructure:"workload,omitempty"`
	Namespace         string `mapstructure:"namespace,omitempty"`
	ResourceName      string `mapstructure:"resource_name,omitempty"`
}

func (k *MetricKey) Init(
	clusterID string,
) {
	hash := xxhash.New()

	hash.Write([]byte(clusterID))
	hash.Write([]byte(k.IntervalStart))
	hash.Write([]byte(k.IntervalEnd))
	hash.Write([]byte(k.MeterDomain))
	hash.Write([]byte(k.MeterKind))
	hash.Write([]byte(k.Label))
	hash.Write([]byte(k.Namespace))
	hash.Write([]byte(k.ResourceName))

	k.MetricID = fmt.Sprintf("%x", hash.Sum64())
}

type MetricBase struct {
	Key              MetricKey              `mapstructure:",squash"`
	AdditionalLabels map[string]interface{} `mapstructure:"additionalLabels"`
	Metrics          map[string]interface{} `mapstructure:"rhmUsageMetrics"`
}

func TimeToReportTimeStr(myTime time.Time) string {
	return myTime.Format(time.RFC3339)
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

func (m *MetricBase) AddAdditionalLabelsFromMap(metrics map[string]interface{}) error {
	if m.AdditionalLabels == nil {
		m.AdditionalLabels = make(map[string]interface{})
	}

	err := mergo.Merge(&m.AdditionalLabels, metrics)
	if err != nil {
		return errors.Wrap(err, "error merging additional labels")
	}

	return nil
}

func (m *MetricBase) AddAdditionalLabels(keysAndValues ...interface{}) error {
	metrics, err := kvToMap(keysAndValues)

	if err != nil {
		return errors.Wrap(err, "error converting to map")
	}

	return m.AddAdditionalLabelsFromMap(metrics)
}

func (m *MetricBase) AddMetrics(keysAndValues ...interface{}) error {
	metrics, err := kvToMap(keysAndValues)

	if err != nil {
		return errors.Wrap(err, "error converting to map")
	}

	if m.Metrics == nil {
		m.Metrics = make(map[string]interface{})
	}

	err = mergo.Merge(&m.Metrics, metrics)

	if err != nil {
		return errors.Wrap(err, "error merging maps")
	}

	return nil
}

func (m *MetricsReport) AddMetrics(metrics ...*MetricBase) error {
	for _, metric := range metrics {
		result := make(map[string]interface{})
		err := mapstructure.Decode(metric, &result)
		if err != nil {
			logger.Error(err, "error adding metric")
			return err
		}
		m.Metrics = append(m.Metrics, result)
	}

	return nil
}

func (m *MetricsReport) AddMetadata(metadata *ReportFlatMetadata) {
	m.Metadata = metadata
}

func (r *ReportMetadata) AddMetricsReport(report *MetricsReport) {
	r.ReportSlices[report.ReportSliceID] = ReportSlicesValue{
		NumberMetrics: len(report.Metrics),
	}
}

func (r *ReportMetadata) UpdateMetricsReport(report *MetricsReport) {
	r.ReportSlices[report.ReportSliceID] = ReportSlicesValue{
		NumberMetrics: len(report.Metrics),
	}
}

func NewReport() *MetricsReport {
	return &MetricsReport{
		ReportSliceID: ReportSliceKey(uuid.New()),
	}
}

func NewReportMetadata(
	source uuid.UUID,
	metadata ReportSourceMetadata,
) *ReportMetadata {
	return &ReportMetadata{
		ReportID:       uuid.New(),
		Source:         source,
		SourceMetadata: metadata,
		ReportSlices:   make(map[ReportSliceKey]ReportSlicesValue),
	}
}

const justDateFormat = "2006-01-02"

func NewMetric(
	pair model.SamplePair,
	matrix *model.SampleStream,
	meterReport *v1alpha1.MeterReportSpec,
	meterDefLabel *common.MeterDefPrometheusLabels,
	templ *ReportTemplater,
	step time.Duration,
	meterType v1beta1.WorkloadType,
	clusterUUID string,
	kvMap map[string]interface{},
) (*MetricBase, error) {
	namespace, _ := getMatrixValue(matrix.Metric, "namespace")

	logger.V(4).Info("kvMap", "map", kvMap)

	meterDef := meterDefLabel
	err := templ.Execute(meterDef, &ReportLabels{
		Label: kvMap,
	})

	if err != nil {
		logger.Error(err, "failed to run template")
		return nil, err
	}

	if meterDef.DisplayName != "" {
		kvMap["display_name"] = meterDef.DisplayName
	}

	if meterDef.MeterDescription != "" {
		kvMap["display_description"] = meterDef.MeterDescription
	}

	var objName string

	switch meterType {
	case v1beta1.WorkloadTypePVC:
		objName, _ = getMatrixValue(matrix.Metric, "persistentvolumeclaim")
	case v1beta1.WorkloadTypePod:
		objName, _ = getMatrixValue(matrix.Metric, "pod")
	case v1beta1.WorkloadTypeService:
		objName, _ = getMatrixValue(matrix.Metric, "service")
	}

	if objName == "" {
		return nil, errors.NewWithDetails("can't find objName", "type", meterType)
	}

	intervalStart := pair.Timestamp.Time().Format(time.RFC3339)
	intervalEnd := pair.Timestamp.Add(step).Time().Format(time.RFC3339)

	if meterDef.DateLabelOverride != "" {
		t, err := time.Parse(time.RFC3339, meterDef.DateLabelOverride)

		if err != nil {
			t2, err2 := time.Parse(justDateFormat, meterDef.DateLabelOverride)

			if err2 != nil {
				return nil, errors.Combine(err, err2)
			}

			t = t2
		}

		intervalStart = t.Format(time.RFC3339)
		intervalEnd = t.Add(step).Format(time.RFC3339)
	}

	key := MetricKey{
		ReportPeriodStart: meterReport.StartTime.UTC().Format(time.RFC3339),
		ReportPeriodEnd:   meterReport.EndTime.UTC().Format(time.RFC3339),
		IntervalStart:     intervalStart,
		IntervalEnd:       intervalEnd,
		MeterDomain:       meterDef.MeterGroup,
		MeterKind:         meterDef.MeterKind,
		Namespace:         namespace,
		ResourceName:      objName,
		Label:             meterDef.Metric,
	}

	logger.V(4).Info("metric", "metric val", meterDef.Metric)

	key.Init(clusterUUID)

	base := &MetricBase{
		Key: key,
	}

	// override value if valueLabelOverride is set
	value := pair.Value.String()
	if meterDef.ValueLabelOverride != "" {
		value = meterDef.ValueLabelOverride
	}

	logger.V(4).Info("adding pair", "metric", matrix.Metric, "pair", pair)
	metricPairs := []interface{}{meterDef.Metric, value}

	err = base.AddAdditionalLabelsFromMap(kvMap)
	if err != nil {
		return nil, err
	}

	err = base.AddMetrics(metricPairs...)

	if err != nil {
		return nil, err
	}

	return base, nil
}
