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

	"github.com/gotidy/ptr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/tests/rectest"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
		name                    = utils.RAZEE_NAME
		namespace               = "redhat-marketplace"
		secretName              = "rhm-operator-secret"
		req                     reconcile.Request
		opts                    []StepOption
		razeeDeployment         marketplacev1alpha1.RazeeDeployment
		razeeDeploymentDeletion marketplacev1alpha1.RazeeDeployment
		namespObj               corev1.Namespace
		console                 *unstructured.Unstructured
		cluster                 *unstructured.Unstructured
		clusterVersion          *unstructured.Unstructured
		secret                  corev1.Secret
		cosReaderKeySecret      corev1.Secret
		configMap               corev1.ConfigMap
		deployment              appsv1.Deployment
		parentRRS3              marketplacev1alpha1.RemoteResourceS3
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
				Enabled:                 true,
				ClusterUUID:             "foo",
				DeploySecretName:        &secretName,
				LegacyUninstallHasRun:   ptr.Bool(true),
				InstallIBMCatalogSource: ptr.Bool(true),
			},
		}
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
			// r.Client = fake.NewClientBuilder().WithScheme(k8sScheme).WithObjects(r.GetGetObjects()...).Build()
			r.SetClient(fake.NewClientBuilder().WithScheme(k8sScheme).WithObjects(r.GetGetObjects()...).Build())
			cfg, err := config.GetConfig()
			Expect(err).To(Succeed())

			factory := manifests.NewFactory(
				cfg,
				k8sScheme,
			)

			r.SetReconciler(&RazeeDeploymentReconciler{
				Client:  r.GetClient(),
				Scheme:  k8sScheme,
				Log:     log,
				cfg:     cfg,
				factory: factory,
			})
			return nil
		}
	})

	It("clean install", func() {
		t := GinkgoT()
		reconcilerTest := NewReconcilerTest(setup,
			&razeeDeployment,
			&secret,
			&namespObj,
			console,
			cluster,
			clusterVersion,
		)
		reconcilerTest.TestAll(t,
			ReconcileStep(opts,
				ReconcileWithUntilDone(true)),
			// Let's do some client checks
			ListStep(opts,
				ListWithObj(&corev1.ConfigMapList{}),
				ListWithFilter(
					client.InNamespace(namespace),
				),
				ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i client.ObjectList) {
					list, ok := i.(*corev1.ConfigMapList)

					assert.Truef(t, ok, "expected operator group list got type %T", i)
					assert.Equal(t, 4, len(list.Items))

					names := []string{}
					for _, cm := range list.Items {
						names = append(names, cm.Name)
					}

					assert.Contains(t, names, utils.WATCH_KEEPER_NON_NAMESPACED_NAME)
					assert.Contains(t, names, utils.WATCH_KEEPER_LIMITPOLL_NAME)
					assert.Contains(t, names, utils.WATCH_KEEPER_CONFIG_NAME)
					assert.Contains(t, names, utils.RAZEE_CLUSTER_METADATA_NAME)
				})),
			ListStep(opts,
				ListWithObj(&corev1.SecretList{}),
				ListWithFilter(
					client.InNamespace(namespace),
				),
				ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i client.ObjectList) {
					list, ok := i.(*corev1.SecretList)

					assert.Truef(t, ok, "expected operator group list got type %T", i)
					assert.Equal(t, 3, len(list.Items))

					names := []string{}
					for _, cm := range list.Items {
						names = append(names, cm.Name)
					}

					assert.Contains(t, names, utils.WATCH_KEEPER_SECRET_NAME)
					assert.Contains(t, names, utils.RHM_OPERATOR_SECRET_NAME)
					assert.Contains(t, names, utils.COS_READER_KEY_NAME)
				})),
			ListStep(opts,
				ListWithObj(&operatorsv1alpha1.CatalogSourceList{}),
				ListWithFilter(
					client.InNamespace(utils.OPERATOR_MKTPLACE_NS),
				),
				ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i client.ObjectList) {
					list, ok := i.(*operatorsv1alpha1.CatalogSourceList)

					assert.Truef(t, ok, "expected CatalogSourceList got type %T", i)

					names := []string{}
					for _, cs := range list.Items {
						names = append(names, cs.Name)
					}

					assert.Contains(t, names, utils.IBM_CATALOGSRC_NAME)
					assert.Contains(t, names, utils.OPENCLOUD_CATALOGSRC_NAME)
				})),
			ReconcileStep(opts,
				ReconcileWithUntilDone(true)),
		)

	})

	It("no secret", func() {
		t := GinkgoT()
		reconcilerTest := NewReconcilerTest(setup, &razeeDeployment, &namespObj)
		reconcilerTest.TestAll(t,
			ReconcileStep(opts,
				ReconcileWithExpectedResults(
					ReconcileResult{}),
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
			&secret,
			&namespObj,
			&razeeDeploymentDeletion,
			&parentRRS3,
			&cosReaderKeySecret,
			&configMap,
			&deployment,
		)

		reconcilerTest.TestAll(t,
			ReconcileStep(opts,
				ReconcileWithUntilDone(true)),
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
		)

		Eventually(func() []marketplacev1alpha1.RemoteResourceS3 {
			list := &marketplacev1alpha1.RemoteResourceS3List{}
			k8sClient.List(context.TODO(), list, client.InNamespace(namespace))

			return list.Items
		}, timeout, interval).Should(HaveLen(0), "system RemoteResourceS3s should be deleted")

		Eventually(func() []corev1.ConfigMap {
			list := &corev1.ConfigMapList{}
			k8sClient.List(context.TODO(), list, client.InNamespace(namespace))

			return list.Items
		}, timeout, interval).Should(HaveLen(0), "system ConfigMaps should be deleted")

		Eventually(func() []corev1.Secret {
			list := &corev1.SecretList{}
			k8sClient.List(context.TODO(), list, client.InNamespace(namespace))

			return list.Items
		}, timeout, interval).Should(HaveLen(0), "system Secrets should be deleted")

		Eventually(func() []appsv1.Deployment {
			list := &appsv1.DeploymentList{}
			k8sClient.List(context.TODO(), list, client.InNamespace(namespace))

			return list.Items
		}, timeout, interval).Should(HaveLen(0), "system Deployments should be deleted")
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
