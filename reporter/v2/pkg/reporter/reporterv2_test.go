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
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("ReporterV2", func() {
	var (
		err           error
		sutv3         *MarketplaceReporter
		sutv2         *MarketplaceReporter
		sutv1         *MarketplaceReporter
		config        *marketplacev1alpha1.MarketplaceConfig
		dir, dir2     string
		uploader      uploaders.Uploader
		generatedData map[string]string
		k8sResources  []interface{}

		cfgv3 *Config
		cfgv2 *Config
		cfgv1 *Config

		v1Writer  schemacommon.ReportWriter
		v1Builder schemacommon.DataBuilder

		v2Writer  schemacommon.ReportWriter
		v2Builder schemacommon.DataBuilder

		v3Writer  schemacommon.ReportWriter
		v3Builder schemacommon.DataBuilder

		startStr = "2020-06-19T00:00:00Z"
		endStr   = "2020-07-19T00:00:00Z"
		start, _ = time.Parse(time.RFC3339, startStr)
		end, _   = time.Parse(time.RFC3339, endStr)

		v1metricIds []string
		v2eventIds  []string
		v3eventIds  []string

		checkTime = WithTransform(func(unIn float64) map[string]int {
			t := time.Unix(int64(unIn), 0).UTC()
			return map[string]int{
				"minute": t.Minute(),
				"second": t.Second(),
			}
		}, Or(Equal(map[string]int{
			"minute": 0,
			"second": 0,
		}), Equal(map[string]int{
			"minute": 0,
			"second": 0,
		})))
	)

	BeforeEach(func() {
		uploader = &uploaders.NoOpUploader{}

		dir, err = os.MkdirTemp("", "report")
		dir2, err = os.MkdirTemp("", "targz")

		Expect(err).To(Succeed())

		cfgv3 = &Config{
			OutputDirectory: dir,
			MetricsPerFile:  ptr.Int(20),
			ReporterSchema:  "v3alpha1",
		}

		cfgv3.SetDefaults()

		cfgv2 = &Config{
			OutputDirectory: dir,
			MetricsPerFile:  ptr.Int(20),
			ReporterSchema:  "v2alpha1",
		}

		cfgv2.SetDefaults()

		cfgv1 = &Config{
			OutputDirectory: dir,
			MetricsPerFile:  ptr.Int(20),
			ReporterSchema:  "v1alpha1",
		}

		cfgv1.SetDefaults()

		config = &marketplacev1alpha1.MarketplaceConfig{
			Spec: marketplacev1alpha1.MarketplaceConfigSpec{
				RhmAccountID: "foo",
				ClusterUUID:  "foo-id",
			},
		}
		config.Status.Conditions.SetCondition(status.Condition{
			Type:    marketplacev1alpha1.ConditionRHMAccountExists,
			Status:  corev1.ConditionTrue,
			Reason:  marketplacev1alpha1.ReasonRHMAccountExists,
			Message: "RHM/Software Central account exists",
		})

		v3Writer, _ = ProvideWriter(cfgv3, config, logger)
		v3Builder, _ = ProvideDataBuilder(cfgv3, logger)

		v2Writer, _ = ProvideWriter(cfgv2, config, logger)
		v2Builder, _ = ProvideDataBuilder(cfgv2, logger)

		v1Writer, _ = ProvideWriter(cfgv1, config, logger)
		v1Builder, _ = ProvideDataBuilder(cfgv1, logger)

		k8sResources, err = getK8sInfrastructureResources(context.Background(), k8sClient)
		Expect(err).ToNot(HaveOccurred())
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
			sutv2 = &MarketplaceReporter{
				PrometheusAPI: prometheus.PrometheusAPI{API: v1api},
				Config:        cfgv2,
				MktConfig:     config,
				report: &marketplacev1alpha1.MeterReport{
					Spec: marketplacev1alpha1.MeterReportSpec{
						StartTime: metav1.Time{Time: start},
						EndTime:   metav1.Time{Time: end},
					},
				},
				reportWriter:      v2Writer,
				schemaDataBuilder: v2Builder,
			}
			sutv3 = &MarketplaceReporter{
				PrometheusAPI: prometheus.PrometheusAPI{API: v1api},
				Config:        cfgv3,
				MktConfig:     config,
				report: &marketplacev1alpha1.MeterReport{
					Spec: marketplacev1alpha1.MeterReportSpec{
						StartTime: metav1.Time{Time: start},
						EndTime:   metav1.Time{Time: end},
					},
				},
				k8sResources:      k8sResources,
				reportWriter:      v3Writer,
				schemaDataBuilder: v3Builder,
			}
		})

		v2rowMatcher := MatchAllKeys(Keys{
			"additionalAttributes": MatchAllKeys(Keys{
				"clusterId":           Equal("foo-id"),
				"group":               Equal("app.partner.metering.com"),
				"kind":                Equal("App"),
				"meter_def_name":      Equal("foo"),
				"meter_def_namespace": Equal("bar"),
				"namespace":           Equal("metering-example-operator"),
				"pod":                 Equal("example-app-pod"),
				"meter_domain":        Equal("apps.partner.metering.com"),
				"meter_version":       Equal("v1"),
				"service":             Equal("example-app-service"),
				"metricType":          Equal("license"),
			}),
			"start":     checkTime,
			"end":       checkTime,
			"eventId":   BeAssignableToTypeOf(""),
			"accountId": Equal("foo"),
			"measuredUsage": MatchAllElements(func(element interface{}) string {
				data := element.(map[string]interface{})
				return data["metricId"].(string)
			}, Elements{
				"rpc_durations_seconds_sum": MatchAllKeys(Keys{
					"metricId": Equal("rpc_durations_seconds_sum"),
					"value":    BeNumerically(">", 0),
					"additionalAttributes": MatchAllKeys(Keys{
						"meter_query": Equal("rpc_durations_seconds_sum"),
					}),
				}),
				"my_query": MatchAllKeys(Keys{
					"metricId": Equal("my_query"),
					"value":    BeNumerically(">", 0),
					"additionalAttributes": MatchAllKeys(Keys{
						"meter_query": Equal("my_query"),
					}),
				}),
			}),
		})

		v3rowMatcher := MatchAllKeys(Keys{
			"start":     checkTime,
			"end":       checkTime,
			"eventId":   BeAssignableToTypeOf(""),
			"accountId": Equal("foo"),
			"group":     Equal("app.partner.metering.com"),
			"kind":      Equal("App"),
			"measuredUsage": MatchAllElements(func(element interface{}) string {
				data := element.(map[string]interface{})
				return data["metricId"].(string)
			}, Elements{
				"rpc_durations_seconds_sum": MatchAllKeys(Keys{
					"metricType":          Equal("license"),
					"pod":                 Equal("example-app-pod"),
					"clusterId":           Equal("foo-id"),
					"metricId":            Equal("rpc_durations_seconds_sum"),
					"value":               BeNumerically(">", 0),
					"meter_def_name":      Equal("foo"),
					"meter_def_namespace": Equal("bar"),
					"namespace":           Equal(`[{"labels":{"swc_saas_ibm_com_testkey":"testval"},"name":"metering-example-operator"}]`),
				}),
				"my_query": MatchAllKeys(Keys{
					"metricType":          Equal("license"),
					"pod":                 Equal("example-app-pod"),
					"clusterId":           Equal("foo-id"),
					"metricId":            Equal("my_query"),
					"value":               BeNumerically(">", 0),
					"meter_def_name":      Equal("foo"),
					"meter_def_namespace": Equal("bar"),
					"namespace":           Equal(`[{"labels":{"swc_saas_ibm_com_testkey":"testval"},"name":"metering-example-operator"}]`),
				}),
			}),
		})

		It("v2 query, build and submit a report", func() {
			By("collecting metrics")
			results, errs, _, err := sutv2.CollectMetrics(context.TODO())

			Expect(err).To(Succeed())
			Expect(errs).To(BeEmpty())
			Expect(results).ToNot(BeEmpty())
			Expect(len(results)).To(Equal(713))

			By("writing report")

			files, err := sutv2.WriteReport(
				uuid.New(),
				results)

			Expect(err).To(Succeed())
			Expect(files).ToNot(BeEmpty())
			Expect(len(files)).To(Equal(37))

			for _, file := range files {
				By(fmt.Sprintf("testing file %s", file))
				Expect(file).To(BeAnExistingFile())
				fileBytes, err := os.ReadFile(file)
				Expect(err).To(Succeed(), "file does not exist")
				data := make(map[string]interface{})
				err = json.Unmarshal(fileBytes, &data)
				Expect(err).To(Succeed(), "file data did not parse to json")

				if strings.Contains(file, "manifest") {
					Expect(data).To(MatchAllKeys(Keys{
						"version": Equal("1"),
						"type":    Equal("accountMetrics"),
					}))
				}

				if !strings.Contains(file, "manifest") {
					id := func(element interface{}) string {
						return "row"
					}

					Expect(data["data"].([]interface{})[0]).To(v2rowMatcher)

					Expect(data).To(MatchAllKeys(Keys{
						// metadata is optional
						"data": MatchElements(id, AllowDuplicates, Elements{
							"row": v2rowMatcher,
						}),
						"metadata": MatchAllKeys(Keys{
							"reportVersion":  Equal("v2alpha1"),
							"rhmAccountId":   Equal("foo"),
							"rhmClusterId":   Equal("foo-id"),
							"rhmEnvironment": Equal("production"),
							"version":        BeAssignableToTypeOf(""),
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
		}, NodeTimeout(time.Duration.Seconds(20)))

		It("v3 query, build and submit a report", func() {
			By("collecting metrics")
			results, errs, _, err := sutv3.CollectMetrics(context.TODO())

			Expect(err).To(Succeed())
			Expect(errs).To(BeEmpty())
			Expect(results).ToNot(BeEmpty())
			Expect(len(results)).To(Equal(713))

			By("writing report")

			files, err := sutv3.WriteReport(
				uuid.New(),
				results)

			Expect(err).To(Succeed())
			Expect(files).ToNot(BeEmpty())
			Expect(len(files)).To(Equal(37))

			for _, file := range files {
				By(fmt.Sprintf("testing file %s", file))
				Expect(file).To(BeAnExistingFile())
				fileBytes, err := os.ReadFile(file)
				Expect(err).To(Succeed(), "file does not exist")
				data := make(map[string]interface{})
				err = json.Unmarshal(fileBytes, &data)
				Expect(err).To(Succeed(), "file data did not parse to json")

				if strings.Contains(file, "manifest") {
					Expect(data).To(MatchAllKeys(Keys{
						"version": Equal("1"),
						"type":    Equal("swcAccountMetrics"),
					}))
				}

				if !strings.Contains(file, "manifest") {
					id := func(element interface{}) string {
						return "row"
					}

					Expect(data["data"].([]interface{})[0]).To(v3rowMatcher)

					Expect(data).To(MatchAllKeys(Keys{
						// metadata is optional
						"data": MatchElements(id, AllowDuplicates, Elements{
							"row": v3rowMatcher,
						}),
						"metadata": MatchAllKeys(Keys{
							"reportVersion": Equal("v3alpha1"),
							"accountId":     Equal("foo"),
							"clusterId":     Equal("foo-id"),
							"environment":   Equal("production"),
							"version":       BeAssignableToTypeOf(""),
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
		}, NodeTimeout(time.Duration.Seconds(20)))
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
			sutv2 = &MarketplaceReporter{
				PrometheusAPI: prometheus.PrometheusAPI{API: v1api},
				Config:        cfgv2,
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
				schemaDataBuilder: v2Builder,
				reportWriter:      v2Writer,
			}
			sutv3 = &MarketplaceReporter{
				PrometheusAPI: prometheus.PrometheusAPI{API: v1api},
				Config:        cfgv3,
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
				k8sResources:      k8sResources,
				schemaDataBuilder: v3Builder,
				reportWriter:      v3Writer,
			}
		})

		v2rowMatcher := MatchAllKeys(Keys{
			"additionalAttributes": MatchAllKeys(Keys{
				"clusterId":           Equal("foo-id"),
				"group":               Equal("apps.partner.metering.com"),
				"kind":                Or(Equal("App"), Equal("App2")),
				"meter_def_name":      Or(Equal("foo"), Equal("foo2")),
				"meter_def_namespace": Equal("bar"),
				"namespace":           Equal("metering-example-operator"),
				"pod":                 Equal("example-app-pod"),
				"meter_domain":        Equal("apps.partner.metering.com"),
				"meter_version":       Equal("v1"),
				"service":             Equal("example-app-service"),
				"metricType":          Equal("license"),
			}),
			"start":     checkTime,
			"end":       checkTime,
			"eventId":   BeAssignableToTypeOf(""),
			"accountId": Equal("foo"),

			"measuredUsage": MatchAllElements(func(element interface{}) string {
				data := element.(map[string]interface{})
				return data["metricId"].(string)
			}, Elements{
				"rpc_durations_seconds_sum": MatchAllKeys(Keys{
					"metricId": Equal("rpc_durations_seconds_sum"),
					"value":    BeNumerically(">", 0),
					"additionalAttributes": MatchAllKeys(Keys{
						"meter_query": Equal("rpc_durations_seconds_sum"),
					}),
				}),
				"rpc_durations_seconds_count": MatchAllKeys(Keys{
					"metricId": Equal("rpc_durations_seconds_count"),
					"value":    BeNumerically(">", 0),
					"additionalAttributes": MatchAllKeys(Keys{
						"meter_query": Equal("my_query"),
					}),
				}),
			}),
		})

		v3rowMatcher := MatchAllKeys(Keys{
			"group":     Equal("apps.partner.metering.com"),
			"kind":      Or(Equal("App"), Equal("App2")),
			"start":     checkTime,
			"end":       checkTime,
			"eventId":   BeAssignableToTypeOf(""),
			"accountId": Equal("foo"),

			"measuredUsage": MatchAllElements(func(element interface{}) string {
				data := element.(map[string]interface{})
				return data["metricId"].(string)
			}, Elements{
				"rpc_durations_seconds_sum": MatchAllKeys(Keys{
					"metricType":          Equal("license"),
					"pod":                 Equal("example-app-pod"),
					"clusterId":           Equal("foo-id"),
					"metricId":            Equal("rpc_durations_seconds_sum"),
					"value":               BeNumerically(">", 0),
					"meter_def_name":      Or(Equal("foo"), Equal("foo2")),
					"meter_def_namespace": Equal("bar"),
					"namespace":           Equal(`[{"labels":{"swc_saas_ibm_com_testkey":"testval"},"name":"metering-example-operator"}]`),
				}),
				"rpc_durations_seconds_count": MatchAllKeys(Keys{
					"metricType":          Equal("license"),
					"pod":                 Equal("example-app-pod"),
					"clusterId":           Equal("foo-id"),
					"metricId":            Equal("rpc_durations_seconds_count"),
					"value":               BeNumerically(">", 0),
					"meter_def_name":      Or(Equal("foo"), Equal("foo2")),
					"meter_def_namespace": Equal("bar"),
					"namespace":           Equal(`[{"labels":{"swc_saas_ibm_com_testkey":"testval"},"name":"metering-example-operator"}]`),
				}),
			}),
		})

		It("v2 query, build and submit a report", func() {
			By("collecting metrics")
			results, errs, _, err := sutv2.CollectMetrics(context.TODO())

			Expect(err).To(Succeed())
			Expect(errs).To(BeEmpty())
			Expect(results).ToNot(BeEmpty())

			By("writing report")

			files, err := sutv2.WriteReport(
				uuid.New(),
				results)

			Expect(err).To(Succeed())
			Expect(files).ToNot(BeEmpty())

			for _, file := range files {
				By(fmt.Sprintf("testing file %s", file))
				Expect(file).To(BeAnExistingFile())
				fileBytes, err := os.ReadFile(file)
				Expect(err).To(Succeed(), "file does not exist")
				data := make(map[string]interface{})
				err = json.Unmarshal(fileBytes, &data)
				Expect(err).To(Succeed(), "file data did not parse to json")

				if strings.Contains(file, "manifest") {
					Expect(data).To(MatchAllKeys(Keys{
						"version": Equal("1"),
						"type":    Equal("accountMetrics"),
					}))
				}

				if !strings.Contains(file, "manifest") {

					id := func(element interface{}) string {
						return "row"
					}

					firstRow := data["data"].([]interface{})[0]

					Expect(firstRow).To(v2rowMatcher)
					Expect(data).To(MatchAllKeys(Keys{
						// metadata is optional
						"data": MatchElements(id, AllowDuplicates, Elements{
							"row": v2rowMatcher,
						}),
						"metadata": MatchAllKeys(Keys{
							"reportVersion":  Equal("v2alpha1"),
							"rhmAccountId":   Equal("foo"),
							"rhmClusterId":   Equal("foo-id"),
							"rhmEnvironment": Equal("production"),
							"version":        BeAssignableToTypeOf(""),
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
		}, NodeTimeout(time.Duration.Seconds(20)))

		It("v3 query, build and submit a report", func() {
			By("collecting metrics")
			results, errs, _, err := sutv3.CollectMetrics(context.TODO())

			Expect(err).To(Succeed())
			Expect(errs).To(BeEmpty())
			Expect(results).ToNot(BeEmpty())

			By("writing report")

			files, err := sutv3.WriteReport(
				uuid.New(),
				results)

			Expect(err).To(Succeed())
			Expect(files).ToNot(BeEmpty())

			for _, file := range files {
				By(fmt.Sprintf("testing file %s", file))
				Expect(file).To(BeAnExistingFile())
				fileBytes, err := os.ReadFile(file)
				Expect(err).To(Succeed(), "file does not exist")
				data := make(map[string]interface{})
				err = json.Unmarshal(fileBytes, &data)
				Expect(err).To(Succeed(), "file data did not parse to json")

				if strings.Contains(file, "manifest") {
					Expect(data).To(MatchAllKeys(Keys{
						"version": Equal("1"),
						"type":    Equal("swcAccountMetrics"),
					}))
				}

				if !strings.Contains(file, "manifest") {

					id := func(element interface{}) string {
						return "row"
					}

					firstRow := data["data"].([]interface{})[0]

					Expect(firstRow).To(v3rowMatcher)
					Expect(data).To(MatchAllKeys(Keys{
						// metadata is optional
						"data": MatchElements(id, AllowDuplicates, Elements{
							"row": v3rowMatcher,
						}),
						"metadata": MatchAllKeys(Keys{
							"reportVersion": Equal("v3alpha1"),
							"accountId":     Equal("foo"),
							"clusterId":     Equal("foo-id"),
							"environment":   Equal("production"),
							"version":       BeAssignableToTypeOf(""),
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
		}, NodeTimeout(time.Duration.Seconds(20)))

	})

	Context("v1 v2 v3 schema match ids", func() {
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

			sutv3 = &MarketplaceReporter{
				PrometheusAPI: prometheus.PrometheusAPI{API: v1api},
				Config:        cfgv3,
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
				k8sResources:      k8sResources,
				schemaDataBuilder: v3Builder,
				reportWriter:      v3Writer,
			}

			sutv2 = &MarketplaceReporter{
				PrometheusAPI: prometheus.PrometheusAPI{API: v1api},
				Config:        cfgv2,
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
				schemaDataBuilder: v2Builder,
				reportWriter:      v2Writer,
			}

			sutv1 = &MarketplaceReporter{
				PrometheusAPI: prometheus.PrometheusAPI{API: v1api},
				Config:        cfgv1,
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

		It("query, build and submit a report", func() {
			By("collecting metrics v3")
			resultsv3, errs, _, err := sutv3.CollectMetrics(context.TODO())

			Expect(err).To(Succeed())
			Expect(errs).To(BeEmpty())
			Expect(resultsv3).ToNot(BeEmpty())

			By("writing report")

			files, err := sutv3.WriteReport(
				uuid.New(),
				resultsv3)

			Expect(err).To(Succeed())
			Expect(files).ToNot(BeEmpty())

			for _, file := range files {
				By(fmt.Sprintf("testing file %s", file))
				Expect(file).To(BeAnExistingFile())
				fileBytes, err := os.ReadFile(file)
				Expect(err).To(Succeed(), "file does not exist")
				data := make(map[string]interface{})
				err = json.Unmarshal(fileBytes, &data)
				Expect(err).To(Succeed(), "file data did not parse to json")

				if !strings.Contains(file, "manifest") {
					datarows := data["data"].([]interface{})
					for _, data := range datarows {
						datamap := data.(map[string]interface{})
						eventId, ok := datamap["eventId"].(string)
						Expect(ok).To(BeTrue(), "eventId not found")
						v3eventIds = append(v3eventIds, eventId)
					}
				}
			}

			By("collecting metrics v2")
			resultsv2, errs, _, err := sutv2.CollectMetrics(context.TODO())

			Expect(err).To(Succeed())
			Expect(errs).To(BeEmpty())
			Expect(resultsv2).ToNot(BeEmpty())

			By("writing report")

			files, err = sutv2.WriteReport(
				uuid.New(),
				resultsv2)

			Expect(err).To(Succeed())
			Expect(files).ToNot(BeEmpty())

			for _, file := range files {
				By(fmt.Sprintf("testing file %s", file))
				Expect(file).To(BeAnExistingFile())
				fileBytes, err := os.ReadFile(file)
				Expect(err).To(Succeed(), "file does not exist")
				data := make(map[string]interface{})
				err = json.Unmarshal(fileBytes, &data)
				Expect(err).To(Succeed(), "file data did not parse to json")

				if !strings.Contains(file, "manifest") {
					datarows := data["data"].([]interface{})
					for _, data := range datarows {
						datamap := data.(map[string]interface{})
						eventId, ok := datamap["eventId"].(string)
						Expect(ok).To(BeTrue(), "eventId not found")
						v2eventIds = append(v2eventIds, eventId)
					}
				}
			}

			By("collecting metrics v1")
			resultsv1, errs, _, err := sutv1.CollectMetrics(context.TODO())

			Expect(err).To(Succeed())
			Expect(errs).To(BeEmpty())
			Expect(resultsv1).ToNot(BeEmpty())

			By("writing report")

			files, err = sutv1.WriteReport(
				uuid.New(),
				resultsv1)

			Expect(err).To(Succeed())
			Expect(files).ToNot(BeEmpty())

			for _, file := range files {
				By(fmt.Sprintf("testing file %s", file))
				Expect(file).To(BeAnExistingFile())
				fileBytes, err := os.ReadFile(file)
				Expect(err).To(Succeed(), "file does not exist")
				data := make(map[string]interface{})
				err = json.Unmarshal(fileBytes, &data)
				Expect(err).To(Succeed(), "file data did not parse to json")

				if !strings.Contains(file, "metadata") {
					metrics := data["metrics"].([]interface{})
					for _, metric := range metrics {
						metricmap := metric.(map[string]interface{})
						metricId, ok := metricmap["metric_id"].(string)
						Expect(ok).To(BeTrue(), "metricId not found")
						v1metricIds = append(v1metricIds, metricId)
					}
				}
			}

			Expect(v2eventIds).Should(ContainElements(v1metricIds))
			Expect(v3eventIds).Should(ContainElements(v2eventIds))

		}, NodeTimeout(time.Duration.Seconds(20)))

	})

	Context("Infrastructure", func() {
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
								MetricType:   "infrastructure",
							},
							{

								Aggregation:  "sum",
								Metric:       "rpc_durations_seconds",
								Query:        "my_query",
								Label:        "rpc_durations_seconds_count",
								WorkloadType: common.WorkloadTypePod,
								MetricType:   "infrastructure",
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
								MetricType:   "infrastructure",
							},
							{

								Aggregation:  "sum",
								Metric:       "rpc_durations_seconds",
								Query:        "my_query",
								Label:        "rpc_durations_seconds_count",
								WorkloadType: common.WorkloadTypePod,
								MetricType:   "infrastructure",
							},
						},
					},
				},
			}

			v1api := getTestAPI(mockResponseRoundTripper(generatedData, meterDefs, start, end))

			sutv3 = &MarketplaceReporter{
				PrometheusAPI: prometheus.PrometheusAPI{API: v1api},
				Config:        cfgv3,
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
				k8sResources:      k8sResources,
				schemaDataBuilder: v3Builder,
				reportWriter:      v3Writer,
			}
		})

		v3rowMatcher := MatchAllKeys(Keys{
			"group":     Equal("apps.partner.metering.com"),
			"kind":      Or(Equal("App"), Equal("App2")),
			"start":     checkTime,
			"end":       checkTime,
			"eventId":   BeAssignableToTypeOf(""),
			"accountId": Equal("foo"),

			"measuredUsage": MatchAllElements(func(element interface{}) string {
				data := element.(map[string]interface{})
				return data["metricId"].(string)
			}, Elements{
				"rpc_durations_seconds_sum": MatchAllKeys(Keys{
					"metricType":          Equal("infrastructure"),
					"pod":                 Equal("example-app-pod"),
					"clusterId":           Equal("foo-id"),
					"metricId":            Equal("rpc_durations_seconds_sum"),
					"value":               BeNumerically(">", 0),
					"meter_def_name":      Or(Equal("foo"), Equal("foo2")),
					"meter_def_namespace": Equal("bar"),
					"k8sResources":        Not(BeEmpty()),
					"namespace":           Equal(`[{"labels":{"swc_saas_ibm_com_testkey":"testval"},"name":"metering-example-operator"}]`),
				}),
				"rpc_durations_seconds_count": MatchAllKeys(Keys{
					"metricType":          Equal("infrastructure"),
					"pod":                 Equal("example-app-pod"),
					"clusterId":           Equal("foo-id"),
					"metricId":            Equal("rpc_durations_seconds_count"),
					"value":               BeNumerically(">", 0),
					"meter_def_name":      Or(Equal("foo"), Equal("foo2")),
					"meter_def_namespace": Equal("bar"),
					"k8sResources":        Not(BeEmpty()),
					"namespace":           Equal(`[{"labels":{"swc_saas_ibm_com_testkey":"testval"},"name":"metering-example-operator"}]`),
				}),
			}),
		})

		It("v3 query, build and submit an infrastructure report", func() {
			By("collecting metrics")
			results, errs, _, err := sutv3.CollectMetrics(context.TODO())

			Expect(err).To(Succeed())
			Expect(errs).To(BeEmpty())
			Expect(results).ToNot(BeEmpty())

			By("writing report")

			files, err := sutv3.WriteReport(
				uuid.New(),
				results)

			Expect(err).To(Succeed())
			Expect(files).ToNot(BeEmpty())

			for _, file := range files {
				By(fmt.Sprintf("testing file %s", file))
				Expect(file).To(BeAnExistingFile())
				fileBytes, err := os.ReadFile(file)
				Expect(err).To(Succeed(), "file does not exist")
				data := make(map[string]interface{})
				err = json.Unmarshal(fileBytes, &data)
				Expect(err).To(Succeed(), "file data did not parse to json")

				if strings.Contains(file, "manifest") {
					Expect(data).To(MatchAllKeys(Keys{
						"version": Equal("1"),
						"type":    Equal("swcAccountMetrics"),
					}))
				}

				if !strings.Contains(file, "manifest") {

					id := func(element interface{}) string {
						return "row"
					}

					firstRow := data["data"].([]interface{})[0]

					Expect(firstRow).To(v3rowMatcher)
					Expect(data).To(MatchAllKeys(Keys{
						// metadata is optional
						"data": MatchElements(id, AllowDuplicates, Elements{
							"row": v3rowMatcher,
						}),
						"metadata": MatchAllKeys(Keys{
							"reportVersion": Equal("v3alpha1"),
							"accountId":     Equal("foo"),
							"clusterId":     Equal("foo-id"),
							"environment":   Equal("production"),
							"version":       BeAssignableToTypeOf(""),
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
		}, NodeTimeout(time.Duration.Seconds(20)))

	})

})
