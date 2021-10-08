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

package controller_test

import (
	"context"
	"time"

	"github.com/blang/semver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

const timeout = time.Second * 100
const longTimeout = time.Second * 200
const jobTimeout = time.Second * 500
const dataServiceTimeout = time.Second * 1200
const interval = time.Second * 3

var _ = Describe("MeterbaseController", func() {
	Context("MeterBase reconcile", func() {
		Context("creating a meterbase", func() {
			It("should create all assets", func(done Done) {
				cm := &corev1.ConfigMap{}
				deployment := &appsv1.Deployment{}
				service := &corev1.Service{}

				By("create prometheus operator")
				Eventually(func() bool {
					result, _ := testHarness.Do(
						context.TODO(),
						GetAction(types.NamespacedName{Name: "serving-certs-ca-bundle", Namespace: Namespace}, cm),
						GetAction(types.NamespacedName{Name: "prometheus-operator", Namespace: Namespace}, deployment),
						GetAction(types.NamespacedName{Name: "prometheus-operator", Namespace: Namespace}, service),
					)
					return result.Is(Continue)
				}, timeout, interval).Should(BeTrue())

				By("creating metric-state")

				deployment = &appsv1.Deployment{}
				service = &corev1.Service{}
				serviceMonitor := &monitoringv1.ServiceMonitor{}

				Eventually(func() bool {
					result, _ := testHarness.Do(
						context.TODO(),
						GetAction(types.NamespacedName{Name: "rhm-metric-state", Namespace: Namespace}, deployment),
						GetAction(types.NamespacedName{Name: "rhm-metric-state-service", Namespace: Namespace}, service),
						GetAction(types.NamespacedName{Name: "rhm-metric-state", Namespace: Namespace}, serviceMonitor),
					)
					return result.Is(Continue)
				}, timeout, interval).Should(BeTrue())

				By("creating additional config secret")

				clusterVersionObj := &openshiftconfigv1.ClusterVersion{}
				secret := &corev1.Secret{}

				// additional-scrape-configs no longer used on 4.6.0+
				Eventually(func() bool {
					result, _ := testHarness.Do(
						context.TODO(),
						GetAction(types.NamespacedName{Name: "version"}, clusterVersionObj),
					)
					if result.Is(NotFound) {
						// Not Openshift, check additional-scrape-configs
						result, _ = testHarness.Do(
							context.TODO(),
							GetAction(types.NamespacedName{Name: "rhm-meterbase-additional-scrape-configs", Namespace: Namespace}, secret),
						)
						return result.Is(Continue)
					} else if result.Is(Continue) {
						// Is Openshift
						parsedVersion460, _ := semver.Make("4.6.0")
						parsedVersion, err := semver.ParseTolerant(clusterVersionObj.Status.Desired.Version)
						if err != nil {
							return false
						}
						if parsedVersion.GTE(parsedVersion460) {
							// 4.6.0+, no additional-scrape-configs
							return result.Is(Continue)
						} else {
							// <4.6.0, check additional-scrape-configs
							result, _ = testHarness.Do(
								context.TODO(),
								GetAction(types.NamespacedName{Name: "rhm-meterbase-additional-scrape-configs", Namespace: Namespace}, secret),
							)
							return result.Is(Continue)
						}
					}
					return result.Is(Continue)
				}, timeout, interval).Should(BeTrue())

				close(done)
			})
		})
	})
})
