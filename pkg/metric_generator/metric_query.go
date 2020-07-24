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

package metric_generator

// import (
//  "time"

//  promapi "github.com/prometheus/client_golang/api"
//  promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
//  "github.com/prometheus/common/model"
//  marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
//  reporter "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/reporter"
//  corev1 "k8s.io/api/core/v1"
//  metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// )

// var (
//  start, _ = time.Parse(time.RFC3339, "2020-04-19T13:00:00Z")
//  end, _   = time.Parse(time.RFC3339, "2020-04-19T16:00:00Z")

//  metricsHost       = "0.0.0.0"
//  metricsPort int32 = 8383

//  storage corev1.ConfigMap
// )

// type Query struct {
//  Metric    string
//  Labels    map[string]string
//  StartTime time.Time
//  // EndTime   time.Time
// }

// func init() {
//  storage = buildStorage()
// }

// /* Goals of the metric_query: retrieve metrics created leveraging kube-state-api

// - Create a client that can communicate with prometheus
// - have a query builder function
// - join existing metrics with our labels
// - create a configMap to store data
// - have a function which queries prometheus
// - save responses in configMap
// */

// // buildPrometheusAPI returns an API that can communicate with our prometheus instance
// func buildPrometheusAPI() (promv1.API, error) {
//  cfg := promapi.Config{
//      Address: "localhost:9090",
//  }

//  promClient, err := promapi.NewClient(cfg)
//  if err != nil {
//      return nil, err
//  }

//  promAPI := promv1.NewAPI(promClient)

//  return promAPI, err
// }

// //buildQuery takes meterdefinition and returns a query that we can submit to prometheus
// func buildQuery(metric string, def marketplacev1alpha1.MeterDefinition) Query {

//  // start, _ := (time.RFC3339, time.Now().String())
//  // end, _ := (time.RFC3339, (time.Now().Add(time.Hour * 2)).String

//  return Query{
//      Metric: metric,
//      Labels: map[string]string{
//          "meter_def_kind":    def.Spec.MeterKind,
//          "meter_def_domain":  def.Spec.MeterDomain,
//          "meter_def_version": def.Spec.MeterVersion,
//      },
//      StartTime: time.Now(),
//      // EndTime:   time.Now().Add(time.Hour * 2),
//  }

// }

// func MatchLabels(results model.Values, query Query) {

//  var res string

//  saveResults(query, res)
// }

// func (q Query) String() string {
//  var res string
//  res = q.Metric
//  for k, v := range q.Labels {
//      res = res + " - " + k + ": " + v
//  }
//  res = res + " - StartTime: " + q.StartTime.String()
//  // res = res + " - EndTime: " + q.EndTime.String()
//  return res
// }

// func submitQuery(promApi promv1.API, query Query) (model.Value, Warnings, error) {
//  res, warnings, err := promApi.Query(context.TODO(), query.Metric, query.StartTime)
//  if err != nil {
//  }
//  MatchLabels(res, query)
// }

// //buildStorage returns a ConfigMap that will store our results (from prometheus api queries)
// func buildStorage(instance *marketplacev1alpha1.MeterDefinition) *corev1.ConfigMap {
//  return &corev1.ConfigMap{
//      ObjectMeta: metav1.ObjectMeta{
//          Name:      "meterdef_results",
//          Namespace: instance.Namespace,
//      },
//  }
// }

// // saveResult saves the result of the PrometheusQuery into the ConfigMap
// // result format: metric_name - meterdef_kind - meterdef_domain - meterdef_version - pod/service_name - namespace - time_start - time_end: result
// func saveResult(query reporter.PromQuery, result string, storage corev1.ConfigMap) {
//  temp := storage.Data
//  temp[query.Metric] = result
//  storage.Data = temp
// }
