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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	utils "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/certificates"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/tests/rectest"
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
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

			setup = func(r *ReconcilerTest) error {
				var log = logf.Log.WithName("certissuer_controller")
				discoveryClient, _ := discovery.NewDiscoveryClientForConfig(cfg)
				r.Client = fake.NewFakeClient(r.GetGetObjects()...)
				inf, _ := config.NewInfrastructure(r.Client, discoveryClient)
				r.Reconciler = &CertIssuerReconciler{
					Client:     r.Client,
					Scheme:     scheme.Scheme,
					Log:        log,
					certIssuer: ci,
					cfg: &config.OperatorConfig{
						Infrastructure: inf,
					},
				}
				return nil
			}

			testConfigMapDataInsertion = func(t GinkgoTInterface) {
				t.Parallel()
				reconcilerTest := NewReconcilerTest(setup, configmap)
				reconcilerTest.TestAll(t,
					ReconcileStep(opts,
						ReconcileWithUntilDone(true)),
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
		)
		testConfigMapDataInsertion(GinkgoT())
	})
})
