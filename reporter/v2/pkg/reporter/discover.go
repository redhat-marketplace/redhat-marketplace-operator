package reporter

import (
	"time"

	"github.com/prometheus/common/model"
)

func getQueries(matrixVals model.Matrix) ([]*meterDefPromQuery, error) {
	results := []*meterDefPromQuery{}

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
				min = v.Timestamp.Time().UTC()
				max = v.Timestamp.Time().UTC().Add(time.Hour)
			}

			if v.Timestamp.Time().Before(min) {
				min = v.Timestamp.Time()
			}
			if v.Timestamp.Time().After(max) {
				max = v.Timestamp.Time()
			}
		}

		max = max.Add(-time.Second)
		promQuery, err := buildPromQuery(matrix.Metric, min, max)

		if err != nil {
			return results, err
		}

		logger.Info("getting query", "query", promQuery.String(), "start", min, "end", max)
		results = append(results, promQuery)
	}

	return results, nil
}
