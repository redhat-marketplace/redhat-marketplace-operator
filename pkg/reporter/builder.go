package reporter

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/imdario/mergo"
	"github.com/mitchellh/mapstructure"
	"github.ibm.com/symposium/redhat-marketplace-operator/pkg/utils"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("report")

type ReportMetadata struct {
	ReportID       uuid.UUID                            `json:"report_id"`
	Source         uuid.UUID                            `json:"source"`
	SourceMetadata ReportSourceMetadata                 `json:"source_metadata"`
	ReportSlices   map[ReportSliceKey]ReportSlicesValue `json:"report_slices"`
}

type ReportSourceMetadata struct {
	RhmClusterID string `json:"rhm_cluster_id"`
	RhmAccountID string `json:"rhm_account_id"`
}

type ReportSliceKey uuid.UUID


func (sliceKey ReportSliceKey) MarshalText() ([]byte, error) {
	return uuid.UUID(sliceKey).MarshalText()
}

func (sliceKey ReportSliceKey) MarshalBinary() ([]byte, error) {
	return uuid.UUID(sliceKey).MarshalBinary()
}

func (sliceKey ReportSliceKey) String() (string) {
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
	ReportPeriodStart string `mapstructure:"report_period_start",json:"report_period_start"`
	ReportPeriodEnd   string `mapstructure:"report_period_end"`
	IntervalStart     string `mapstructure:"interval_start"`
	IntervalEnd       string `mapstructure:"interval_end"`
	MeterDomain       string `mapstructure:"domain"`
	MeterKind         string `mapstructure:"kind"`
	MeterVersion      string `mapstructure:"version"`
}

type MetricBase struct {
	Key     MetricKey `mapstructure:",squash"`
	metrics map[string]interface{}
}

func TimeToReportTimeStr(myTime time.Time) string {
	return myTime.Format(time.RFC3339)
}

func (m *MetricBase) AddMetrics(keysAndValues ...interface{}) error {
	if len(keysAndValues)%2 != 0 {
		err := fmt.Errorf("keyAndValues must be a length of 2")
		log.Error(err, "invalid arguments")
	}

	chunks := utils.ChunkBy(keysAndValues, 2)

	if m.metrics == nil {
		m.metrics = make(map[string]interface{})
	}

	for _, chunk := range chunks {
		key := chunk[0]
		value := chunk[1]

		keyStr, ok := key.(string)

		if !ok {
			err := fmt.Errorf("key is not a string %t", key)
			log.Error(err, "error converting key")
			return err
		}

		m.metrics[keyStr] = value
	}

	return nil
}

func (m *MetricsReport) AddMetrics(metrics ...*MetricBase) error {
	for _, metric := range metrics {
		result := make(map[string]interface{})
		err := mapstructure.Decode(metric, &result)
		if err != nil {
			log.Error(err, "error adding metric")
			return err
		}
		err = mergo.Merge(&result, metric.metrics)
		if err != nil {
			log.Error(err, "error adding metric")
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
