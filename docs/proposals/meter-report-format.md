# Meter Report Format

https://github.ibm.com/symposium/OfferingLifecycleAPI/blob/master/metering.md#api

New version would be v1beta1
Move additional fields to additionalLabels
Change measuredUsage to an array with { metricId, value }

```go
metadata:
  dataVersion: v1alpha1
metadata:
  dataVersion: v1beta1

"metric_id":           Equal("id"),    // event_id?
"report_period_start": Equal("start"),
"report_period_end":   Equal("end"),
"interval_start":      Equal("istart"),
"interval_end":        Equal("iend"),
"additionalLabels": MatchAllKeys(Keys{
  "domain":        Equal("test"), // <- extra data
  "kind":          Equal("foo"),
  "version":       Equal("v1"),
  "resource_name": Equal("foo"),
  "namespace":     Equal("bar"),
  "workload":      Equal("awesome"),
  "extra":         Equal("g"),
 }),
// "measuredUsage": [
// {
//   "metricId": "A123456",
//   "value": 100
// },
```
