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

package marketplace

import (
	"context"
	"time"

	"github.com/gotidy/ptr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/patch"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/tests/rectest"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Testing with Ginkgo", func() {
	var setup func(r *ReconcilerTest) error
	var (
		name            = utils.RAZEE_NAME
		namespace       = "redhat-marketplace"
		secretName      = "rhm-operator-secret"
		req             reconcile.Request
		opts            []StepOption
		razeeDeployment marketplacev1alpha1.RazeeDeployment
		//razeeDeploymentLegacyUninstall marketplacev1alpha1.RazeeDeployment
		razeeDeploymentDeletion marketplacev1alpha1.RazeeDeployment
		namespObj               corev1.Namespace
		//console                        *unstructured.Unstructured
		//cluster                        *unstructured.Unstructured
		//clusterVersion                 *unstructured.Unstructured
		//secret             corev1.Secret
		cosReaderKeySecret corev1.Secret
		configMap          corev1.ConfigMap
		deployment         appsv1.Deployment
		parentRRS3         marketplacev1alpha1.RemoteResourceS3
	)

	BeforeEach(func() {

		name = utils.RAZEE_NAME
		namespace = "redhat-marketplace"
		secretName = "rhm-operator-secret"

		req = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}
		opts = []StepOption{
			WithRequest(req),
		}
		razeeDeployment = marketplacev1alpha1.RazeeDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				UID:       types.UID(uuid.NewUUID()),
			},
			Spec: marketplacev1alpha1.RazeeDeploymentSpec{
				Enabled:               true,
				ClusterUUID:           "foo",
				DeploySecretName:      &secretName,
				LegacyUninstallHasRun: ptr.Bool(true),
			},
		}
		/*
			razeeDeploymentLegacyUninstall = marketplacev1alpha1.RazeeDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: marketplacev1alpha1.RazeeDeploymentSpec{
					Enabled:          true,
					ClusterUUID:      "foo",
					DeploySecretName: &secretName,
				},
			}
		*/
		razeeDeploymentDeletion = marketplacev1alpha1.RazeeDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Finalizers: []string{
					utils.RAZEE_DEPLOYMENT_FINALIZER,
				},
			},
			Spec: marketplacev1alpha1.RazeeDeploymentSpec{
				Enabled:          true,
				ClusterUUID:      "foo",
				DeploySecretName: &secretName,
				TargetNamespace:  &namespace,
			},
		}

		namespObj = corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		/*
				console = &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "config.openshift.io/v1",
						"kind":       "Console",
						"metadata": map[string]interface{}{
							"name": "cluster",
						},
					},
				}

				cluster = &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "config.openshift.io/v1",
						"kind":       "Infrastructure",
						"metadata": map[string]interface{}{
							"name": "cluster",
						},
					},
				}

				clusterVersion = &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "config.openshift.io/v1",
						"kind":       "ClusterVersion",
						"metadata": map[string]interface{}{
							"name": "version",
						},
					},
				}

			secret = corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rhm-operator-secret",
					Namespace: namespace,
				},
				Data: map[string][]byte{
					utils.IBM_COS_READER_KEY_FIELD: []byte("rhm-cos-reader-key"),
					utils.IBM_COS_URL_FIELD:        []byte("rhm-cos-url"),
					utils.BUCKET_NAME_FIELD:        []byte("bucket-name"),
					utils.RAZEE_DASH_ORG_KEY_FIELD: []byte("razee-dash-org-key"),
					utils.CHILD_RRS3_YAML_FIELD:    []byte("childRRS3-filename"),
					utils.RAZEE_DASH_URL_FIELD:     []byte("razee-dash-url"),
					utils.FILE_SOURCE_URL_FIELD:    []byte("file-source-url"),
				},
			}
		*/

		cosReaderKeySecret = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      utils.COS_READER_KEY_NAME,
				Namespace: namespace,
			},
			Data: map[string][]byte{
				utils.IBM_COS_READER_KEY_FIELD: []byte("rhm-cos-reader-key"),
			},
		}
		configMap = corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      utils.WATCH_KEEPER_CONFIG_NAME,
				Namespace: namespace,
			},
		}

		deployment = appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      utils.RHM_WATCHKEEPER_DEPLOYMENT_NAME,
				Namespace: namespace,
			},
		}
		parentRRS3 = marketplacev1alpha1.RemoteResourceS3{
			ObjectMeta: metav1.ObjectMeta{
				Name:      utils.PARENT_RRS3_RESOURCE_NAME,
				Namespace: namespace,
			},
		}

		setup = func(r *ReconcilerTest) error {
			var log = logf.Log.WithName("razee_controller")
			r.SetClient(fake.NewFakeClientWithScheme(k8sScheme, r.GetGetObjects()...))
			cfg, err := config.GetConfig()
			Expect(err).To(Succeed())

			factory := manifests.NewFactory(
				cfg,
				k8sScheme,
			)

			r.SetReconciler(&RazeeRRS3DeploymentReconciler{
				Client:  r.GetClient(),
				Scheme:  k8sScheme,
				Log:     log,
				CC:      reconcileutils.NewClientCommand(r.GetClient(), k8sScheme, log),
				cfg:     cfg,
				factory: factory,
				patcher: patch.RHMDefaultPatcher,
			})
			return nil
		}
	})

	It("no secret", func() {
		t := GinkgoT()
		reconcilerTest := NewReconcilerTest(setup, &razeeDeployment, &namespObj)
		reconcilerTest.TestAll(t,
			ReconcileStep(opts,
				ReconcileWithExpectedResults(
					RequeueResult,
					RequeueResult,
					RequeueResult,
					RequeueAfterResult(time.Second*60)),
			))
	})

	It("bad name", func() {
		t := GinkgoT()
		razeeDeploymentLocalDeployment := razeeDeployment.DeepCopy()
		razeeDeploymentLocalDeployment.Name = "foo"
		reconcilerTest := NewReconcilerTest(setup, razeeDeploymentLocalDeployment, &namespObj)
		reconcilerTest.TestAll(t,
			ReconcileStep(opts,
				ReconcileWithExpectedResults(
					DoneResult,
				),
			))
	})

	It("full uninstall", func() {
		t := GinkgoT()
		reconcilerTest := NewReconcilerTest(setup,
			&namespObj,
			&razeeDeploymentDeletion,
			&parentRRS3,
			&cosReaderKeySecret,
			&configMap,
			&deployment,
		)

		reconcilerTest.TestAll(t,
			GetStep(opts,
				GetWithObj(&marketplacev1alpha1.RazeeDeployment{}),
				GetWithNamespacedName(name, namespace),
				GetWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i client.Object) {
					if i != nil {
						r.Client.Delete(context.TODO(), i)
					}
				}),
			),
			ReconcileStep(opts,
				ReconcileWithUntilDone(true)),
			ListStep(opts,
				ListWithObj(&marketplacev1alpha1.RemoteResourceS3List{}),
				ListWithFilter(
					client.InNamespace(namespace),
				),
				ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i client.ObjectList) {
					list, ok := i.(*marketplacev1alpha1.RemoteResourceS3List)
					assert.Truef(t, ok, "expected RemoteResourceS3List got type %T", i)
					assert.Equal(t, 0, len(list.Items))
				})),
			// Split Controller does not manage the ConfigMaps
			ListStep(opts,
				ListWithObj(&corev1.ConfigMapList{}),
				ListWithFilter(
					client.InNamespace(namespace),
				),
				ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i client.ObjectList) {
					list, ok := i.(*corev1.ConfigMapList)
					assert.Truef(t, ok, "expected configMap list got type %T", i)
					assert.Equal(t, 1, len(list.Items))
				})),
			// Split Controller does not manage the Secrets
			ListStep(opts,
				ListWithObj(&corev1.SecretList{}),
				ListWithFilter(
					client.InNamespace(namespace),
				),
				ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i client.ObjectList) {
					list, ok := i.(*corev1.SecretList)

					assert.Truef(t, ok, "expected secret list got type %T", i)
					assert.Equal(t, 1, len(list.Items))
				})),
			// Split Controller does not manage the watch keeper deployment
			ListStep(opts,
				ListWithObj(&appsv1.DeploymentList{}),
				ListWithFilter(
					client.InNamespace(namespace),
				),
				ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i client.ObjectList) {
					list, ok := i.(*appsv1.DeploymentList)

					assert.Truef(t, ok, "expected deployment list got type %T", i)
					assert.Equal(t, 1, len(list.Items))
				})),
		)
	})
})

var _ = Describe("isMapStringByteEqual", func() {
	It("should check equality", func() {
		d1 := map[string][]byte{
			"foo": []byte("bar"),
		}
		d2 := map[string][]byte{
			"foo": []byte("bar"),
		}

		Expect(isMapStringByteEqual(d1, d2)).To(BeTrue())
		Expect(isMapStringByteEqual(d2, d1)).To(BeTrue())

		d2 = map[string][]byte{
			"foo": []byte("bar"),
			"bar": []byte("bar"),
		}

		Expect(isMapStringByteEqual(d1, d2)).To(BeFalse())
		Expect(isMapStringByteEqual(d2, d1)).To(BeFalse())

		d2 = map[string][]byte{
			"bar": []byte("bar"),
		}

		Expect(isMapStringByteEqual(d1, d2)).To(BeFalse())
		Expect(isMapStringByteEqual(d2, d1)).To(BeFalse())

		d1 = map[string][]byte{
			"foo": []byte("bar"),
		}
		d2 = map[string][]byte{
			"foo": []byte("bar2"),
		}

		Expect(isMapStringByteEqual(d1, d2)).To(BeFalse())
		Expect(isMapStringByteEqual(d2, d1)).To(BeFalse())

		d1 = map[string][]byte{
			"foo":  []byte("bar"),
			"foo2": []byte("bar2"),
		}
		d2 = map[string][]byte{
			"foo": []byte("bar"),
		}

		Expect(isMapStringByteEqual(d1, d2)).To(BeFalse())
		Expect(isMapStringByteEqual(d2, d1)).To(BeFalse())
	})
})
