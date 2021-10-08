import "time"

// Release notes: initial release
#V1: {
	_#sourceMetadata: {
		"rhmClusterId":   string
		"rhmAccountId":   string
		"rhmEnvironment": string
		"version":        string
		"reportVersion":  string
	}

	_#metricDetailed: {
		label: string
		value: string
		labelSet?: {
			string: _
		}
	}

	_#metric: {
		"metric_id":           string
		"report_period_start": string
		"report_period_end":   string
		"interval_start":      string
		"interval_end":        string
		"domain":              string
		"kind":                string
		"version"?:            string
		"workload"?:           string
		"resource_namespace"?: string
		"resource_name"?:      string
		"unit"?:               string
		"additionalLabels": {
			string: _
		}
		"rhmUsageMetrics": {
			string: _
		}
		"rhmUsageMetricsDetails": [_#metricDetailed]
		"rhmUsageMetricsDetailedSummary"?: {
			"totalMetricCount": int
		}
	}

	_#reportSliceKey: string

	#reportMetadata: {
		"report_id":       string
		"source":          string
		"source_metadata": _#sourceMetadata
		"report_slices": {
			_#reportSliceKey: {
				"number_metrics": int
			}
		}
	}

	#slice: {
		"report_slice_id": string
		"metadata":        _#sourceMetadata
		"metrics": [_#metric]
	}
}

#V2: {

}
