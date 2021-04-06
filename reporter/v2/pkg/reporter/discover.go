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

package reporter

import (
	"time"

	"emperror.dev/errors"
	"github.com/prometheus/common/model"
	"k8s.io/apimachinery/pkg/types"
)

func getQueries(matrixVals model.Matrix) (map[types.NamespacedName][]*meterDefPromQuery, []error) {
	results := make(map[types.NamespacedName][]*meterDefPromQuery)
	errs := []error{}

	for _, matrix := range matrixVals {
		name, _ := getMatrixValue(matrix.Metric, "name")
		namespace, _ := getMatrixValue(matrix.Metric, "namespace")
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
				max = v.Timestamp.Time().Add(time.Hour)
			}

			start, end := v.Timestamp.Time(), v.Timestamp.Time().Add(time.Hour)
			if start.Before(min) {
				min = start
			}
			if end.After(max) {
				max = end
			}
		}

		max = max.UTC()
		min = min.UTC()
		promQuery, err := buildPromQuery(matrix.Metric, min, max)

		if err != nil {
			errs = append(errs, errors.WrapWithDetails(err, "failed to build query", "meterdef_name", name, "meterdef_namespace", namespace))
			continue
		}

		logger.Info("getting query", "query", promQuery.String(), "start", min, "end", max)

		if v, ok := results[promQuery.query.MeterDef]; ok {
			v = append(v, promQuery)
		} else {
			results[promQuery.query.MeterDef] = []*meterDefPromQuery{promQuery}
		}
	}

	return results, errs
}
