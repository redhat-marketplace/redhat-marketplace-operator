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
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gotidy/ptr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"github.com/google/uuid"
	schemacommon "github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter/schema/common"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/uploaders"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Reporter", func() {
	var (
		err           error
		sut           *MarketplaceReporter
		config        *marketplacev1alpha1.MarketplaceConfig
		dir, dir2     string
		uploader      uploaders.Uploader
		generatedData map[string]string
		cfg           *Config

		v1Writer  schemacommon.ReportWriter
		v1Builder schemacommon.DataBuilder

		startStr = "2020-06-19T00:00:00Z"
		endStr   = "2020-07-19T00:00:00Z"
		start, _ = time.Parse(time.RFC3339, startStr)
		end, _   = time.Parse(time.RFC3339, endStr)
	)

	BeforeEach(func() {
		uploader = &uploaders.NoOpUploader{}

		dir, err = ioutil.TempDir("", "report")
		dir2, err = ioutil.TempDir("", "targz")

		Expect(err).To(Succeed())

		cfg = &Config{
			OutputDirectory: dir,
			MetricsPerFile:  ptr.Int(20),
			ReporterSchema:  "v1alpha1",
		}

		cfg.SetDefaults()

		config = &marketplacev1alpha1.MarketplaceConfig{
			Spec: marketplacev1alpha1.MarketplaceConfigSpec{
				RhmAccountID: "foo",
				ClusterUUID:  "foo-id",
			},
		}

		v1Writer, _ = ProvideWriter(cfg, config, logger)
		v1Builder, _ = ProvideDataBuilder(cfg, logger)
	})

	Context("with templates", func() {
		BeforeEach(func() {
			generatedData = map[string]string{
				"simple_query": GenerateRandomData(start, end, []string{"rpc_durations_seconds_sum", "my_query"}),
			}

			meterDefs := []v1beta1.MeterDefinition{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "bar",
						UID:       types.UID("a"),
					},
					Spec: v1beta1.MeterDefinitionSpec{
						Group: "app.partner.metering.com",
						Kind:  "App",
						ResourceFilters: []v1beta1.ResourceFilter{
							{
								Namespace: &v1beta1.NamespaceFilter{UseOperatorGroup: true},
								OwnerCRD: &v1beta1.OwnerCRDFilter{
									GroupVersionKind: common.GroupVersionKind{
										APIVersion: "apps.partner.metering.com/v1",
										Kind:       "App",
									},
								},
								WorkloadType: common.WorkloadTypePod,
							},
						},
						Meters: []v1beta1.MeterWorkload{
							{
								Aggregation:  "sum",
								Query:        "simple_query",
								Metric:       "rpc_durations_seconds",
								Label:        "{{ .Label.meter_query }}",
								WorkloadType: common.WorkloadTypePod,
								Description:  "{{ .Label.meter_domain | lower }} description",
							},
						},
					},
				},
			}

			v1api := getTestAPI(mockResponseRoundTripper(generatedData, meterDefs, start, end))

			sut = &MarketplaceReporter{
				PrometheusAPI: prometheus.PrometheusAPI{API: v1api},
				Config:        cfg,
				MktConfig:     config,
				report: &marketplacev1alpha1.MeterReport{
					Spec: marketplacev1alpha1.MeterReportSpec{
						StartTime: metav1.Time{Time: start},
						EndTime:   metav1.Time{Time: end},
					},
				},
				reportWriter:      v1Writer,
				schemaDataBuilder: v1Builder,
			}
		})

		rowMatcher := MatchAllKeys(Keys{
			"additionalLabels": MatchKeys(IgnoreExtras, Keys{
				"namespace":           Equal("metering-example-operator"),
				"pod":                 Equal("example-app-pod"),
				"meter_domain":        Equal("apps.partner.metering.com"),
				"meter_version":       Equal("v1"),
				"service":             Equal("example-app-service"),
				"display_description": Equal("apps.partner.metering.com description"),
			}),
			"domain":              Or(Equal("app.partner.metering.com"), Equal("app2.partner.metering.com"), Equal("app3.partner.metering.com")),
			"interval_start":      HavePrefix("2020-"),
			"interval_end":        HavePrefix("2020-"),
			"metric_id":           BeAssignableToTypeOf(""),
			"report_period_end":   Equal(endStr),
			"kind":                Or(Equal("App"), Equal("App2"), Equal("App3")),
			"resource_namespace":  Equal("metering-example-operator"),
			"report_period_start": Equal(startStr),
			"resource_name":       Equal("example-app-pod"),
			"workload":            Equal("rpc_durations_seconds"),
			"rhmUsageMetrics": MatchAllKeys(Keys{
				"my_query":                  BeAssignableToTypeOf(""),
				"rpc_durations_seconds_sum": BeAssignableToTypeOf(""),
			}),
			"rhmUsageMetricsDetailed": MatchAllElements(func(element interface{}) string {
				data := element.(map[string]interface{})
				return data["label"].(string)
			}, Elements{
				"my_query": MatchAllKeys(Keys{
					"label": Equal("my_query"),
					"value": BeAssignableToTypeOf(""),
					"labelSet": MatchAllKeys(Keys{
						"meter_query": Equal("my_query"),
					}),
				}),
				"rpc_durations_seconds_sum": MatchAllKeys(Keys{
					"label": Equal("rpc_durations_seconds_sum"),
					"value": BeAssignableToTypeOf(""),
					"labelSet": MatchAllKeys(Keys{
						"meter_query": Equal("rpc_durations_seconds_sum"),
					}),
				}),
			}),
			"rhmUsageMetricsDetailedSummary": MatchKeys(IgnoreExtras, Keys{
				"totalMetricCount": Equal(2.0),
			}),
		})

		It("query, build and submit a report", func() {
			By("collecting metrics")
			results, errs, _, err := sut.CollectMetrics(context.TODO())

			Expect(err).To(Succeed())
			Expect(errs).To(BeEmpty())
			Expect(results).ToNot(BeEmpty())
			Expect(len(results)).To(Equal(713))

			By("writing report")

			files, err := sut.WriteReport(
				uuid.New(),
				results)

			Expect(err).To(Succeed())
			Expect(files).ToNot(BeEmpty())
			Expect(len(files)).To(Equal(37))

			for _, file := range files {
				By(fmt.Sprintf("testing file %s", file))
				Expect(file).To(BeAnExistingFile())
				fileBytes, err := ioutil.ReadFile(file)
				Expect(err).To(Succeed(), "file does not exist")
				data := make(map[string]interface{})
				err = json.Unmarshal(fileBytes, &data)
				Expect(err).To(Succeed(), "file data did not parse to json")

				if strings.Contains(file, "metadata") {
					Expect(data).To(MatchAllKeys(Keys{
						"report_id": BeAssignableToTypeOf(""),
						"source":    BeAssignableToTypeOf(""),
						"source_metadata": MatchAllKeys(Keys{
							"rhmClusterId":   BeAssignableToTypeOf(""),
							"rhmAccountId":   BeAssignableToTypeOf(""),
							"rhmEnvironment": BeAssignableToTypeOf(""),
							"version":        BeAssignableToTypeOf(""),
							"reportVersion":  BeAssignableToTypeOf(""),
						}),
						"report_slices": WithTransform(func(obj interface{}) interface{} {
							data := obj.(map[string]interface{})
							slice := []interface{}{}

							for _, v := range data {
								slice = append(slice, v)
							}

							return slice
						}, MatchElementsWithIndex(IndexIdentity, IgnoreExtras, Elements{
							"0": MatchAllKeys(Keys{
								"number_metrics": BeNumerically(">", 0),
							}),
						})),
					}))
				}

				if !strings.Contains(file, "metadata") {
					id := func(element interface{}) string {
						return "row"
					}

					Expect(data["metrics"].([]interface{})[0]).To(rowMatcher)

					Expect(data).To(MatchAllKeys(Keys{
						"report_slice_id": Not(BeEmpty()),
						"metadata": MatchKeys(IgnoreExtras, Keys{
							"reportVersion": Equal("v1alpha1"),
							"version":       BeAssignableToTypeOf(""),
						}),
						"metrics": MatchElements(id, AllowDuplicates, Elements{
							"row": rowMatcher,
						}),
					}))
				}
			}

			dirPath := filepath.Dir(files[0])
			fileName := fmt.Sprintf("%s/test-upload.tar.gz", dir2)

			Expect(fileName).ToNot(BeAnExistingFile())

			By(fmt.Sprintf("targz the file %s", fileName))
			Expect(TargzFolder(dirPath, fileName)).To(Succeed())
			Expect(fileName).To(BeAnExistingFile())

			By("uploading file")

			r, _ := os.Open(fileName)
			defer r.Close()
			_, err = uploader.UploadFile(context.TODO(), fileName, r)
			Expect(err).To(Succeed())
		}, 20)
	})

	Context("without templates", func() {
		BeforeEach(func() {
			generatedData = map[string]string{
				"rpc_durations_seconds_sum": GenerateRandomData(start, end, []string{"rpc_durations_seconds_sum"}),
				"my_query":                  GenerateRandomData(start, end, []string{"my_query"}),
			}

			meterDefs := []v1beta1.MeterDefinition{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "bar",
					},
					Spec: v1beta1.MeterDefinitionSpec{
						Group: "apps.partner.metering.com",
						Kind:  "App",
						ResourceFilters: []v1beta1.ResourceFilter{
							{
								Namespace: &v1beta1.NamespaceFilter{UseOperatorGroup: true},
								OwnerCRD: &v1beta1.OwnerCRDFilter{
									GroupVersionKind: common.GroupVersionKind{
										APIVersion: "apps.partner.metering.com/v1",
										Kind:       "App",
									},
								},
								WorkloadType: common.WorkloadTypePod,
							},
						},
						Meters: []v1beta1.MeterWorkload{
							{
								Aggregation:  "sum",
								Metric:       "rpc_durations_seconds",
								Query:        "rpc_durations_seconds_sum",
								Label:        "rpc_durations_seconds_sum",
								WorkloadType: common.WorkloadTypePod,
							},
							{

								Aggregation:  "sum",
								Metric:       "rpc_durations_seconds",
								Query:        "my_query",
								Label:        "rpc_durations_seconds_count",
								WorkloadType: common.WorkloadTypePod,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo2",
						Namespace: "bar",
					},
					Spec: v1beta1.MeterDefinitionSpec{
						Group: "apps.partner.metering.com",
						Kind:  "App2",
						ResourceFilters: []v1beta1.ResourceFilter{
							{
								Namespace: &v1beta1.NamespaceFilter{UseOperatorGroup: true},
								OwnerCRD: &v1beta1.OwnerCRDFilter{
									GroupVersionKind: common.GroupVersionKind{
										APIVersion: "apps.partner.metering.com/v1",
										Kind:       "App",
									},
								},
								WorkloadType: common.WorkloadTypePod,
							},
						},
						Meters: []v1beta1.MeterWorkload{
							{
								Aggregation:  "sum",
								Metric:       "rpc_durations_seconds",
								Query:        "rpc_durations_seconds_sum",
								Label:        "rpc_durations_seconds_sum",
								WorkloadType: common.WorkloadTypePod,
							},
							{

								Aggregation:  "sum",
								Metric:       "rpc_durations_seconds",
								Query:        "my_query",
								Label:        "rpc_durations_seconds_count",
								WorkloadType: common.WorkloadTypePod,
							},
						},
					},
				},
			}

			v1api := getTestAPI(mockResponseRoundTripper(generatedData, meterDefs, start, end))

			sut = &MarketplaceReporter{
				PrometheusAPI: prometheus.PrometheusAPI{API: v1api},
				Config:        cfg,
				MktConfig:     config,
				meterDefinitions: MeterDefinitionReferences{
					{
						Name:      "foo",
						Namespace: "bar",
						Spec:      &meterDefs[0].Spec,
					},
				},
				report: &marketplacev1alpha1.MeterReport{
					Spec: marketplacev1alpha1.MeterReportSpec{
						StartTime: metav1.Time{Time: start},
						EndTime:   metav1.Time{Time: end},
					},
				},
				reportWriter:      v1Writer,
				schemaDataBuilder: v1Builder,
			}
		})

		rowMatcher := MatchAllKeys(Keys{
			"additionalLabels": MatchKeys(IgnoreExtras, Keys{
				"namespace":     Equal("metering-example-operator"),
				"pod":           Equal("example-app-pod"),
				"meter_domain":  Equal("apps.partner.metering.com"),
				"meter_version": Equal("v1"),
				"service":       Equal("example-app-service"),
			}),
			"domain":              Or(Equal("apps.partner.metering.com")),
			"interval_start":      HavePrefix("2020-"),
			"interval_end":        HavePrefix("2020-"),
			"metric_id":           BeAssignableToTypeOf(""),
			"report_period_end":   Equal(endStr),
			"kind":                Or(Equal("App"), Equal("App2"), Equal("App3")),
			"report_period_start": Equal(startStr),
			"resource_namespace":  Equal("metering-example-operator"),
			"resource_name":       Equal("example-app-pod"),
			"workload":            Equal("rpc_durations_seconds"),
			"rhmUsageMetrics": MatchAllKeys(Keys{
				"rpc_durations_seconds_count": BeAssignableToTypeOf(""),
				"rpc_durations_seconds_sum":   BeAssignableToTypeOf(""),
			}),
			"rhmUsageMetricsDetailed": MatchAllElements(func(element interface{}) string {
				data := element.(map[string]interface{})
				return data["label"].(string)
			}, Elements{
				"rpc_durations_seconds_count": MatchAllKeys(Keys{
					"label": Equal("rpc_durations_seconds_count"),
					"value": BeAssignableToTypeOf(""),
					"labelSet": MatchAllKeys(Keys{
						"meter_query": Equal("my_query"),
					}),
				}),
				"rpc_durations_seconds_sum": MatchAllKeys(Keys{
					"label": Equal("rpc_durations_seconds_sum"),
					"value": BeAssignableToTypeOf(""),
					"labelSet": MatchAllKeys(Keys{
						"meter_query": Equal("rpc_durations_seconds_sum"),
					}),
				}),
			}),
			"rhmUsageMetricsDetailedSummary": MatchKeys(IgnoreExtras, Keys{
				"totalMetricCount": Equal(2.0),
			}),
		})

		It("query, build and submit a report", func() {
			By("collecting metrics")
			results, errs, _, err := sut.CollectMetrics(context.TODO())

			Expect(err).To(Succeed())
			Expect(errs).To(BeEmpty())
			Expect(results).ToNot(BeEmpty())

			By("writing report")

			files, err := sut.WriteReport(
				uuid.New(),
				results)

			Expect(err).To(Succeed())
			Expect(files).ToNot(BeEmpty())

			for _, file := range files {
				By(fmt.Sprintf("testing file %s", file))
				Expect(file).To(BeAnExistingFile())
				fileBytes, err := ioutil.ReadFile(file)
				Expect(err).To(Succeed(), "file does not exist")
				data := make(map[string]interface{})
				err = json.Unmarshal(fileBytes, &data)
				Expect(err).To(Succeed(), "file data did not parse to json")

				if strings.Contains(file, "metadata") {
					Expect(data).To(MatchAllKeys(Keys{
						"report_id": BeAssignableToTypeOf(""),
						"source":    BeAssignableToTypeOf(""),
						"source_metadata": MatchAllKeys(Keys{
							"rhmClusterId":   BeAssignableToTypeOf(""),
							"rhmAccountId":   BeAssignableToTypeOf(""),
							"rhmEnvironment": BeAssignableToTypeOf(""),
							"version":        BeAssignableToTypeOf(""),
							"reportVersion":  BeAssignableToTypeOf(""),
						}),
						"report_slices": WithTransform(func(obj interface{}) interface{} {
							data := obj.(map[string]interface{})
							slice := []interface{}{}

							for _, v := range data {
								slice = append(slice, v)
							}

							return slice
						}, MatchElementsWithIndex(IndexIdentity, IgnoreExtras, Elements{
							"0": MatchAllKeys(Keys{
								"number_metrics": BeNumerically(">", 0),
							}),
						})),
					}))
				}

				if !strings.Contains(file, "metadata") {

					id := func(element interface{}) string {
						return "row"
					}

					firstRow := data["metrics"].([]interface{})[0]

					Expect(firstRow).To(rowMatcher)
					Expect(data).To(MatchAllKeys(Keys{
						"report_slice_id": Not(BeEmpty()),
						"metadata": MatchKeys(IgnoreExtras, Keys{
							"reportVersion": Equal("v1alpha1"),
							"version":       BeAssignableToTypeOf(""),
						}),
						"metrics": MatchElements(id, AllowDuplicates, Elements{
							"row": rowMatcher,
						}),
					}))
				}
			}

			dirPath := filepath.Dir(files[0])
			fileName := fmt.Sprintf("%s/test-upload.tar.gz", dir2)

			Expect(fileName).ToNot(BeAnExistingFile())

			By(fmt.Sprintf("targz the file %s", fileName))
			Expect(TargzFolder(dirPath, fileName)).To(Succeed())
			Expect(fileName).To(BeAnExistingFile())

			By("uploading file")

			r, _ := os.Open(fileName)
			defer r.Close()
			_, err = uploader.UploadFile(context.TODO(), fileName, r)
			Expect(err).To(Succeed())
		}, 20)
	})
})
