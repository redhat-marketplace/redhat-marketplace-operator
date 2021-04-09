// Copyright 2021 IBM Corp.
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

package marketplace

import (
	"fmt"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	utils "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/certificates"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/tests/rectest"
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("CertIssuerController", func() {
	var (
		namespacedName = types.NamespacedName{
			Name:      "test",
			Namespace: "test",
		}
		err error
		ci  *utils.CertIssuer
	)

	BeforeEach(func() {
		var (
			log = logf.Log.WithName("certissuer_controller")
		)
		ci, err = utils.NewCertIssuer(log)
		Expect(err).To(Succeed())
	})

	It("certificate issuer controller", func() {
		var (
			req = reconcile.Request{
				NamespacedName: namespacedName,
			}
			opts = []StepOption{
				WithRequest(req),
			}

			rhmpService = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespacedName.Namespace,
					Name:      "redhat-marketplace-controller-manager-service",
					UID:       types.UID("123"),
				},
				TypeMeta: metav1.TypeMeta{
					Kind: "Service",
				},
			}

			configmap = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespacedName.Namespace,
					Name:      namespacedName.Name,
					Annotations: map[string]string{
						"service.beta.openshift.io/inject-cabundle": "true",
					},
				},
				Data: map[string]string{
					"service-ca.crt": "",
				},
			}

			outdatedSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespacedName.Namespace,
					Name:      "prometheus-operator-tls",
					Annotations: map[string]string{
						"service.beta.openshift.io/inject-cabundle": "true",
						"certificate/ExpiresOn":                     fmt.Sprintf("%v", time.Now().Unix()),
					},
				},
				Data: map[string][]byte{
					"tls.crt": []byte("abcdefgh"),
				},
			}

			setup = func(r *ReconcilerTest) error {
				var log = logf.Log.WithName("certissuer_controller")
				r.Client = fake.NewFakeClient(r.GetGetObjects()...)
				r.Reconciler = &CertIssuerReconciler{
					Client:     r.Client,
					Scheme:     scheme.Scheme,
					Log:        log,
					certIssuer: ci,
				}
				return nil
			}

			testConfigMapDataInsertion = func(t GinkgoTInterface) {
				t.Parallel()
				reconcilerTest := NewReconcilerTest(setup, rhmpService, configmap)
				reconcilerTest.TestAll(t,
					ReconcileStep(opts,
						ReconcileWithExpectedResults(
							RequeueAfterResult(time.Hour*24),
						),
					),
					ListStep(opts,
						ListWithObj(&corev1.ConfigMapList{}),
						ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i runtime.Object) {
							list, ok := i.(*corev1.ConfigMapList)
							Expect(ok).To(BeTrue())
							for _, cm := range list.Items {
								Expect(len(cm.Data)).Should(BeNumerically(">", 0))
							}
						}),
					),
				)
			}

			testSecretCreation = func(t GinkgoTInterface) {
				t.Parallel()
				reconcilerTest := NewReconcilerTest(setup, rhmpService, configmap)
				reconcilerTest.TestAll(t,
					ReconcileStep(opts,
						ReconcileWithExpectedResults(
							RequeueAfterResult(time.Hour*24),
						),
					),
					ListStep(opts,
						ListWithObj(&corev1.SecretList{}),
						ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i runtime.Object) {
							list, ok := i.(*corev1.SecretList)
							Expect(ok).To(BeTrue())
							tlsSecrets := []string{
								"prometheus-operator-tls",
								"rhm-metric-state-tls",
								"rhm-prometheus-meterbase-tls",
								"rhmp-ca-tls",
							}
							names := []string{}
							for _, s := range list.Items {
								names = append(names, s.GetName())
								Expect(len(s.Data)).Should(BeNumerically(">", 0))
								Expect(len(s.Annotations["certificate/ExpiresOn"])).Should(BeNumerically(">", 0))
							}
							Expect(reflect.DeepEqual(tlsSecrets, names)).To(BeTrue())
						}),
					),
				)
			}

			testRotatingSecret = func(t GinkgoTInterface) {
				t.Parallel()
				reconcilerTest := NewReconcilerTest(setup, rhmpService, configmap, outdatedSecret)
				reconcilerTest.TestAll(t,
					ReconcileStep(opts,
						ReconcileWithExpectedResults(
							RequeueAfterResult(time.Hour*24),
						),
					),
					ListStep(opts,
						ListWithObj(&corev1.SecretList{}),
						ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i runtime.Object) {
							list, ok := i.(*corev1.SecretList)
							Expect(ok).To(BeTrue())

							for _, s := range list.Items {
								Expect(len(s.Annotations["certificate/ExpiresOn"])).Should(BeNumerically(">", 0))
								if s.GetName() == "prometheus-operator-tls" {
									Expect(s.Data["tls.crt"]).ToNot(Equal([]byte("abcdefgh")))
									Expect(len(s.Data["tls.key"])).Should(BeNumerically(">", 0))
								}
							}
						}),
					),
				)
			}
		)

		testConfigMapDataInsertion(GinkgoT())
		testSecretCreation(GinkgoT())
		testRotatingSecret(GinkgoT())
	})
})
