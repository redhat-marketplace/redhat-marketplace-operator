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
	"fmt"
	"time"

	"emperror.dev/errors"
	"github.com/cespare/xxhash"
	"github.com/google/uuid"
	"github.com/imdario/mergo"
	"github.com/mitchellh/mapstructure"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
)

type ReportEnvironment string

const (
	ReportProductionEnv ReportEnvironment = "production"
	ReportSandboxEnv    ReportEnvironment = "sandbox"
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

type ReportSourceMetadata struct {
	RhmClusterID   string            `json:"rhmClusterId"`
	RhmAccountID   string            `json:"rhmAccountId"`
	RhmEnvironment ReportEnvironment `json:"rhmEnvironment,omitempty"`
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
}

type MetricKey struct {
	MetricID          string `mapstructure:"metric_id"`
	ReportPeriodStart string `mapstructure:"report_period_start"`
	ReportPeriodEnd   string `mapstructure:"report_period_end"`
	IntervalStart     string `mapstructure:"interval_start"`
	IntervalEnd       string `mapstructure:"interval_end"`
	MeterDomain       string `mapstructure:"domain"`
	MeterKind         string `mapstructure:"kind"`
	MeterVersion      string `mapstructure:"version"`
}

func (k *MetricKey) Init(ClusterID, unit, namespace string) {
	hash := xxhash.New()

	hash.Write([]byte(ClusterID))
	hash.Write([]byte(k.IntervalStart))
	hash.Write([]byte(k.IntervalEnd))
	hash.Write([]byte(k.MeterDomain))
	hash.Write([]byte(k.MeterKind))
	hash.Write([]byte(unit))
	hash.Write([]byte(namespace))

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

func (m *MetricBase) AddAdditionalLabels(keysAndValues ...interface{}) error {
	metrics, err := kvToMap(keysAndValues)

	if err != nil {
		return errors.Wrap(err, "error converting to map")
	}

	if m.AdditionalLabels == nil {
		m.AdditionalLabels = make(map[string]interface{})
	}

	err = mergo.Merge(&m.AdditionalLabels, metrics)
	if err != nil {
		return errors.Wrap(err, "error merging additional labels")
	}

	return nil
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
