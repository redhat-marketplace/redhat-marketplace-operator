package reporter

import (
	"time"

	"emperror.dev/errors"
	"github.com/prometheus/common/model"
)

func getQueries(matrixVals model.Matrix) ([]*meterDefPromQuery, []error) {
	results := []*meterDefPromQuery{}
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
		results = append(results, promQuery)
	}

	return results, errs
}
