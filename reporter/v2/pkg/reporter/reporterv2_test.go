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
	"bytes"

	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gotidy/ptr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"github.com/google/uuid"
	schemacommon "github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter/schema/common"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("ReporterV2", func() {
	var (
		err           error
		sut           *MarketplaceReporter
		sutv1         *MarketplaceReporter
		config        *marketplacev1alpha1.MarketplaceConfig
		dir, dir2     string
		uploader      Uploader
		generatedData map[string]string
		cfg           *Config
		cfgv1         *Config

		v1Writer  schemacommon.ReportWriter
		v1Builder schemacommon.DataBuilder

		v2Writer  schemacommon.ReportWriter
		v2Builder schemacommon.DataBuilder

		startStr = "2020-06-19T00:00:00Z"
		endStr   = "2020-07-19T00:00:00Z"
		start, _ = time.Parse(time.RFC3339, startStr)
		end, _   = time.Parse(time.RFC3339, endStr)

		metricIds []string
		eventIds  []string
	)

	BeforeEach(func() {
		uploader, err = NewRedHatInsightsUploader(&RedHatInsightsUploaderConfig{
			URL:             "https://cloud.redhat.com",
			ClusterID:       "2858312a-ff6a-41ae-b108-3ed7b12111ef",
			OperatorVersion: "1.0.0",
			Token:           "token",
		})
		Expect(err).To(Succeed())

		uploader.(*RedHatInsightsUploader).client.Transport = &stubRoundTripper{
			roundTrip: func(req *http.Request) *http.Response {
				headers := make(http.Header)
				headers.Add("content-type", "text")

				Expect(req.URL.String()).To(Equal(fmt.Sprintf(uploadURL, uploader.(*RedHatInsightsUploader).URL)), "url does not match expected")
				_, meta, _ := req.FormFile("file")
				header := meta.Header

				Expect(header.Get("Content-Type")).To(Equal(mktplaceFileUploadType))
				Expect(header.Get("Content-Disposition")).To(
					HavePrefix(`form-data; name="%s"; filename="%s"`, "file", "test-upload.tar.gz"))
				Expect(req.Header.Get("Content-Type")).To(HavePrefix("multipart/form-data; boundary="))
				Expect(req.Header.Get("User-Agent")).To(HavePrefix("marketplace-operator"))

				return &http.Response{
					StatusCode: 202,
					// Send response to be tested
					Body: ioutil.NopCloser(bytes.NewBuffer([]byte{})),
					// Must be set to non-nil value or it panics
					Header: headers,
				}
			},
		}

		dir, err = ioutil.TempDir("", "report")
		dir2, err = ioutil.TempDir("", "targz")

		Expect(err).To(Succeed())

		cfg = &Config{
			OutputDirectory: dir,
			MetricsPerFile:  ptr.Int(20),
			ReporterSchema:  "v2alpha1",
		}

		cfg.SetDefaults()

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

		v2Writer, _ = ProvideWriter(cfg, config, logger)
		v2Builder, _ = ProvideDataBuilder(cfg, logger)

		v1Writer, _ = ProvideWriter(cfgv1, config, logger)
		v1Builder, _ = ProvideDataBuilder(cfgv1, logger)
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
				reportWriter:      v2Writer,
				schemaDataBuilder: v2Builder,
			}
		})

		rowMatcher := MatchAllKeys(Keys{
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
			"start":     BeNumerically(">", 0),
			"end":       BeNumerically(">", 0),
			"eventId":   BeAssignableToTypeOf(""),
			"groupName": Equal("app.partner.metering.com"),
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

					Expect(data["data"].([]interface{})[0]).To(rowMatcher)

					Expect(data).To(MatchAllKeys(Keys{
						// metadata is optional
						"data": MatchElements(id, AllowDuplicates, Elements{
							"row": rowMatcher,
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

			Expect(uploader.UploadFile(fileName)).To(Succeed())
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
				schemaDataBuilder: v2Builder,
				reportWriter:      v2Writer,
			}
		})

		rowMatcher := MatchAllKeys(Keys{
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
			"start":     BeNumerically(">", 0),
			"end":       BeNumerically(">", 0),
			"eventId":   BeAssignableToTypeOf(""),
			"groupName": Equal("apps.partner.metering.com"),
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

					Expect(firstRow).To(rowMatcher)
					Expect(data).To(MatchAllKeys(Keys{
						// metadata is optional
						"data": MatchElements(id, AllowDuplicates, Elements{
							"row": rowMatcher,
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

			Expect(uploader.UploadFile(fileName)).To(Succeed())
		}, 20)
	})

	Context("v1 & v2 schema match ids", func() {
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
			By("collecting metrics v2")
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

				if !strings.Contains(file, "manifest") {
					datarows := data["data"].([]interface{})
					for _, data := range datarows {
						datamap := data.(map[string]interface{})
						eventId, ok := datamap["eventId"].(string)
						Expect(ok).To(BeTrue(), "eventId not found")
						eventIds = append(eventIds, eventId)
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
				fileBytes, err := ioutil.ReadFile(file)
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
						metricIds = append(metricIds, metricId)
					}
				}
			}

			Expect(eventIds).Should(ContainElements(metricIds))

		}, 20)

	})

})