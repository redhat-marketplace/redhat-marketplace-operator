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

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

const timeout = time.Second * 180
const interval = time.Second * 3

var _ = Describe("MeterbaseController", func() {
	BeforeEach(func() {
		Expect(testHarness.BeforeAll()).To(Succeed())
	})

	AfterEach(func() {
		Expect(testHarness.AfterAll()).To(Succeed())
	})

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
						GetAction(types.NamespacedName{Name: "operator-certs-ca-bundle", Namespace: Namespace}, cm),
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

				secret := &corev1.Secret{}

				Eventually(func() bool {
					result, _ := testHarness.Do(
						context.TODO(),
						GetAction(types.NamespacedName{Name: "rhm-meterbase-additional-scrape-configs", Namespace: Namespace}, secret),
					)
					return result.Is(Continue)
				}, timeout, interval).Should(BeTrue())

				close(done)
			}, 180)
		})
	})
})
