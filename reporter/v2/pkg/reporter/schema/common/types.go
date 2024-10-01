// Copyright 2021 IBM Corp.
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

package common

import (
	"time"

	"github.com/google/uuid"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
)

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

// Time always prints in UTC time.
type Time time.Time

func (t Time) MarshalText() ([]byte, error) {
	return []byte(t.String()), nil
}

func (t *Time) UnmarshalText(data []byte) error {
	t1, err := time.Parse(time.RFC3339, string(data))

	if err != nil {
		return err
	}

	t2 := Time(t1)
	t = &t2
	return nil
}

func (t Time) String() string {
	return time.Time(t).UTC().Format(time.RFC3339)
}

type DataBuilder interface {
	New() SchemaMetricBuilder
}

type DataBuilderFunc func() SchemaMetricBuilder

func (d DataBuilderFunc) New() SchemaMetricBuilder {
	return d()
}

type SchemaMetricBuilder interface {
	SetAccountID(accountID string)
	SetClusterID(clusterID string)
	AddMeterDefinitionLabels(meterDef *common.MeterDefPrometheusLabelsTemplated)
	SetReportInterval(start, end Time)
	Build() (interface{}, error)
	SetNamespaceLabels(nsLabels map[string]map[string]string)
}

type ReportWriter interface {
	WriteReport(
		source uuid.UUID,
		metrics map[string]SchemaMetricBuilder,
		outputDirectory string,
		partitionSize int,
	) (files []string, err error)
}
