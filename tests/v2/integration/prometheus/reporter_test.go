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

package prometheus_test

// import reporter code
// build a sample report using the prometheus data
// from license server and demonstrate correct
// data collection

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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"github.com/google/uuid"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

var _ = Describe("Reporter", func() {
	var (
		sut    *reporter.MarketplaceReporter
		config *marketplacev1alpha1.MarketplaceConfig
		cfg    *reporter.Config
		dir    string

		server     *PrometheusDockerTest
		start, end time.Time
	)

	Context("robin", func() {
		BeforeEach(func() {
			cwd, err := os.Getwd()
			Expect(err).To(Succeed())

			path := filepath.Join(cwd, "..", "..", "data", "prometheus-robin-datatest-20210608", "robinmeterdef.yaml")
			f, err := os.Open(path)
			Expect(err).To(Succeed())

			dir = filepath.Join(cwd, "..", "..", ".test-data")

			err = os.Mkdir(dir, 0700)
			Expect(err).To(Or(Succeed(), MatchError(os.ErrExist)))

			mdef := &v1beta1.MeterDefinition{}
			err = yaml.NewYAMLOrJSONDecoder(f, 100).Decode(&mdef)
			Expect(err).To(Succeed())

			v1api, err := prometheus.NewPrometheusAPIForReporter(&prometheus.PrometheusAPISetup{
				RunLocal: true,
			})

			Expect(err).To(Succeed())

			refs := []v1beta1.MeterDefinitionReference{}
			spec := mdef.Spec
			refs = append(refs, v1beta1.MeterDefinitionReference{
				Name:            mdef.Name,
				Namespace:       mdef.Namespace,
				ResourceVersion: mdef.ResourceVersion,
				Spec:            &spec,
			})

			cfg = &reporter.Config{
				OutputDirectory: dir,
				MetricsPerFile:  ptr.Int(200),
			}

			cfg.SetDefaults()

			config = &marketplacev1alpha1.MarketplaceConfig{
				Spec: marketplacev1alpha1.MarketplaceConfigSpec{
					RhmAccountID: "foo",
					ClusterUUID:  "foo-id",
				},
			}

			start = MustTime("2021-06-17T00:00:00Z")
			end = MustTime("2021-06-18T00:00:00Z")

			sut, _ = reporter.NewMarketplaceReporter(
				cfg,
				&marketplacev1alpha1.MeterReport{
					Spec: marketplacev1alpha1.MeterReportSpec{
						StartTime:                 metav1.Time{Time: start},
						EndTime:                   metav1.Time{Time: end},
						MeterDefinitionReferences: refs,
					},
				},
				config,
				&prometheus.PrometheusAPI{API: v1api},
				refs,
			)

			server = &PrometheusDockerTest{
				DataPath: filepath.Join("data", "prometheus-robin-datatest-20210608"),
			}

			Eventually(func() error {
				return server.Start()
			}, time.Second*60, time.Second*5).Should(Succeed())
		})

		AfterEach(func() {
			server.Stop()
		})

		It("query, build and submit a report", func() {
			By("collecting metrics")
			results, errs, _, err := sut.CollectMetrics(context.TODO())

			Expect(err).To(Succeed())
			Expect(errs).To(BeEmpty())
			Expect(results).ToNot(BeEmpty())
			Expect(len(results)).To(Equal(153))

			By("writing report")

			files, err := sut.WriteReport(
				uuid.New(),
				results)

			Expect(err).To(Succeed())
			Expect(files).ToNot(BeEmpty())
			Expect(len(files)).To(Equal(2))

			for _, file := range files {
				By(fmt.Sprintf("testing file %s", file))
				Expect(file).To(BeAnExistingFile())

				if !strings.Contains(file, "metadata") {
					fileBytes, err := ioutil.ReadFile(file)
					Expect(err).To(Succeed(), "file does not exist")
					data := make(map[string]interface{})
					err = json.Unmarshal(fileBytes, &data)
					Expect(err).To(Succeed(), "file data did not parse to json")
					Expect(data["metrics"]).To(HaveLen(153))

					id := func(element interface{}) string {
						data := element.(map[string]interface{})
						idString := fmt.Sprintf("%v", data["metric_id"])
						return idString
					}

					Expect(data).To(MatchKeys(IgnoreExtras, Keys{
						"report_slice_id": Not(BeEmpty()),
						"metrics": MatchElements(id, IgnoreExtras, Elements{
							"67e73b7fef59a255": MatchKeys(IgnoreExtras, Keys{
								"rhmUsageMetrics": MatchAllKeys(Keys{
									"node_hour2": Equal("1"),
								}),
							}),
						}),
					}))
				}
			}
		}, 20)
	})

	Context("licenseserver", func() {
		BeforeEach(func() {
			cwd, err := os.Getwd()
			Expect(err).To(Succeed())

			path := filepath.Join(cwd, "..", "..", "data", "prometheus-license-multiple-meterdefs-20210331000000", "licenseservermdefs.yaml")
			f, err := os.Open(path)
			Expect(err).To(Succeed())

			dir = filepath.Join(cwd, "..", "..", ".test-data")

			err = os.Mkdir(dir, 0700)
			Expect(err).To(Or(Succeed(), MatchError(os.ErrExist)))

			meterDefs := &v1beta1.MeterDefinitionList{}
			err = yaml.NewYAMLOrJSONDecoder(f, 100).Decode(&meterDefs)
			Expect(err).To(Succeed())

			v1api, err := prometheus.NewPrometheusAPIForReporter(&prometheus.PrometheusAPISetup{
				RunLocal: true,
			})

			Expect(err).To(Succeed())

			refs := []v1beta1.MeterDefinitionReference{}
			for _, mdef := range meterDefs.Items {
				spec := mdef.Spec
				refs = append(refs, v1beta1.MeterDefinitionReference{
					Name:            mdef.Name,
					Namespace:       mdef.Namespace,
					ResourceVersion: mdef.ResourceVersion,
					Spec:            &spec,
				})
			}

			cfg = &reporter.Config{
				OutputDirectory: dir,
				MetricsPerFile:  ptr.Int(20),
			}

			cfg.SetDefaults()

			config = &marketplacev1alpha1.MarketplaceConfig{
				Spec: marketplacev1alpha1.MarketplaceConfigSpec{
					RhmAccountID: "foo",
					ClusterUUID:  "foo-id",
				},
			}

			start = MustTime("2021-03-27T00:00:00Z")
			end = MustTime("2021-03-28T00:00:00Z")

			sut, _ = reporter.NewMarketplaceReporter(
				cfg,
				&marketplacev1alpha1.MeterReport{
					Spec: marketplacev1alpha1.MeterReportSpec{
						StartTime:                 metav1.Time{Time: start},
						EndTime:                   metav1.Time{Time: end},
						MeterDefinitionReferences: refs,
					},
				},
				config,
				&prometheus.PrometheusAPI{API: v1api},
				refs,
			)

			server = &PrometheusDockerTest{
				DataPath: filepath.Join("data", "prometheus-license-multiple-meterdefs-20210331000000"),
			}

			Eventually(func() error {
				return server.Start()
			}, time.Second*60, time.Second*5).Should(Succeed())
		})

		AfterEach(func() {
			server.Stop()
		})

		It("query, build and submit a report", func() {
			By("collecting metrics")
			results, errs, _, err := sut.CollectMetrics(context.TODO())

			Expect(err).To(Succeed())
			Expect(errs).To(BeEmpty())
			Expect(results).ToNot(BeEmpty())
			Expect(len(results)).To(Equal(11))

			By("writing report")

			files, err := sut.WriteReport(
				uuid.New(),
				results)

			Expect(err).To(Succeed())
			Expect(files).ToNot(BeEmpty())
			Expect(len(files)).To(Equal(2))

			for _, file := range files {
				By(fmt.Sprintf("testing file %s", file))
				Expect(file).To(BeAnExistingFile())

				if !strings.Contains(file, "metadata") {
					fileBytes, err := ioutil.ReadFile(file)
					Expect(err).To(Succeed(), "file does not exist")
					data := make(map[string]interface{})
					err = json.Unmarshal(fileBytes, &data)
					Expect(err).To(Succeed(), "file data did not parse to json")

					id := func(element interface{}) string {
						data := element.(map[string]interface{})
						idString := fmt.Sprintf("%v", data["metric_id"])
						return idString
					}

					Expect(data["metrics"]).To(HaveLen(11))

					Expect(data).To(MatchKeys(IgnoreExtras, Keys{
						"report_slice_id": Not(BeEmpty()),
						"metrics": MatchElements(id, IgnoreExtras, Elements{
							"8a422c1b08ccf647": MatchKeys(IgnoreExtras, Keys{
								"additionalLabels": MatchKeys(IgnoreExtras, Keys{
									"productId": Equal("eb9998dcc5d24e3eb5b6fb488f750fe2"),
								}),
								"rhmUsageMetrics": MatchAllKeys(Keys{
									"IBM Cloud Pak for Data":               Equal("1"),
									"IBM Cloud Pak for Data Control Plane": Equal("10"),
								}),
								"rhmUsageMetricsDetailed": MatchAllElements(func(element interface{}) string {
									data := element.(map[string]interface{})
									return data["label"].(string)
								}, Elements{
									"IBM Cloud Pak for Data": MatchAllKeys(Keys{
										"label": Equal("IBM Cloud Pak for Data"),
										"value": Equal("1"),
										"labelSet": MatchAllKeys(Keys{
											"productName": Equal("IBM Cloud Pak for Data"),
											"value":       Equal("1"),
										}),
									}),
									"IBM Cloud Pak for Data Control Plane": MatchAllKeys(Keys{
										"label": Equal("IBM Cloud Pak for Data Control Plane"),
										"value": Equal("10"),
										"labelSet": MatchAllKeys(Keys{
											"productName": Equal("IBM Cloud Pak for Data Control Plane"),
											"value":       Equal("10"),
										}),
									}),
								}),
							}),
						}),
					}))
				}
			}
		}, 20)
	})
})
