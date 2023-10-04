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
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	razeev1alpha2 "github.com/redhat-marketplace/redhat-marketplace-operator/deployer/v2/api/razee/v1alpha2"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	// . "github.com/onsi/gomega/gstruct"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	// "github.com/stretchr/testify/assert"

	// operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"

	// razeev1alpha2 "github.com/redhat-marketplace/redhat-marketplace-operator/deployer/v2/api/razee/v1alpha2"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"

	// "github.com/stretchr/testify/assert"
	// appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	// "sigs.k8s.io/controller-runtime/pkg/client"
	// "sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Testing with Ginkgo", func() {
	// idFn := func(element interface{}) string {
	// 	return fmt.Sprintf("%v", element)
	// }
	var (
		name                    = utils.RAZEE_NAME
		secretName              = "rhm-operator-secret"
		razeeDeployment         marketplacev1alpha1.RazeeDeployment
		razeeDeploymentDeletion marketplacev1alpha1.RazeeDeployment
		// namespObj      corev1.Namespace
		// console        *unstructured.Unstructured
		console *openshiftconfigv1.Console
		// cluster        *unstructured.Unstructured
		// clusterVersion *unstructured.Unstructured
		secret corev1.Secret
		// cosReaderKeySecret      corev1.Secret
		// configMap               corev1.ConfigMap
		// deployment              appsv1.Deployment
		// parentRRS3              razeev1alpha2.RemoteResource
	)

	BeforeEach(func() {

		name = utils.RAZEE_NAME
		// namespace = "redhat-marketplace"
		secretName = "rhm-operator-secret"

		// req = reconcile.Request{
		// 	NamespacedName: types.NamespacedName{
		// 		Name:      name,
		// 		Namespace: namespace,
		// 	},
		// }

		razeeDeployment = marketplacev1alpha1.RazeeDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: operatorNamespace,
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
				Namespace: operatorNamespace,
				Finalizers: []string{
					utils.RAZEE_DEPLOYMENT_FINALIZER,
				},
			},
			Spec: marketplacev1alpha1.RazeeDeploymentSpec{
				Enabled:          true,
				ClusterUUID:      "foo",
				DeploySecretName: &secretName,
				TargetNamespace:  ptr.String(operatorNamespace),
			},
		}

		// console = &unstructured.Unstructured{
		// 	Object: map[string]interface{}{
		// 		"apiVersion": "config.openshift.io/v1",
		// 		"kind":       "Console",
		// 		"metadata": map[string]interface{}{
		// 			"name": "cluster",
		// 		},
		// 	},
		// }

		console = &openshiftconfigv1.Console{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster",
				Namespace: operatorNamespace,
			},
		}

		// cluster = &unstructured.Unstructured{
		// 	Object: map[string]interface{}{
		// 		"apiVersion": "config.openshift.io/v1",
		// 		"kind":       "Infrastructure",
		// 		"metadata": map[string]interface{}{
		// 			"name": "cluster",
		// 		},
		// 	},
		// }

		// clusterVersion = &unstructured.Unstructured{
		// 	Object: map[string]interface{}{
		// 		"apiVersion": "config.openshift.io/v1",
		// 		"kind":       "ClusterVersion",
		// 		"metadata": map[string]interface{}{
		// 			"name": "version",
		// 		},
		// 	},
		// }

		secret = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rhm-operator-secret",
				Namespace: operatorNamespace,
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

		// cosReaderKeySecret = corev1.Secret{
		// 	ObjectMeta: metav1.ObjectMeta{
		// 		Name:      utils.COS_READER_KEY_NAME,
		// 		Namespace: namespace,
		// 	},
		// 	Data: map[string][]byte{
		// 		utils.IBM_COS_READER_KEY_FIELD: []byte("rhm-cos-reader-key"),
		// 	},
		// }
		// configMap = corev1.ConfigMap{
		// 	ObjectMeta: metav1.ObjectMeta{
		// 		Name:      utils.WATCH_KEEPER_CONFIG_NAME,
		// 		Namespace: namespace,
		// 	},
		// }

		// deployment = appsv1.Deployment{
		// 	ObjectMeta: metav1.ObjectMeta{
		// 		Name:      utils.RHM_WATCHKEEPER_DEPLOYMENT_NAME,
		// 		Namespace: namespace,
		// 	},
		// }
		// parentRRS3 = razeev1alpha2.RemoteResource{
		// 	ObjectMeta: metav1.ObjectMeta{
		// 		Name:      utils.PARENT_REMOTE_RESOURCE_NAME,
		// 		Namespace: namespace,
		// 	},
		// }

		// setup = func(r *ReconcilerTest) error {
		// 	var log = logf.Log.WithName("razee_controller")
		// 	// r.Client = fake.NewClientBuilder().WithScheme(k8sScheme).WithObjects(r.GetGetObjects()...).Build()
		// 	r.SetClient(fake.NewClientBuilder().WithScheme(k8sScheme).WithObjects(r.GetGetObjects()...).Build())
		// 	cfg, err := config.GetConfig()
		// 	Expect(err).To(Succeed())

		// 	factory := manifests.NewFactory(
		// 		cfg,
		// 		k8sScheme,
		// 	)

		// 	r.SetReconciler(&RazeeDeploymentReconciler{
		// 		Client:  r.GetClient(),
		// 		Scheme:  k8sScheme,
		// 		Log:     log,
		// 		cfg:     cfg,
		// 		factory: factory,
		// 	})
		// 	return nil
		// }

		marketplaceconfig := utils.BuildMarketplaceConfigCR(operatorNamespace, "account-id")
		marketplaceconfig.Spec.ClusterUUID = "test"
		marketplaceconfig.Spec.IsDisconnected = ptr.Bool(true)
		marketplaceconfig.Spec.ClusterName = "test-cluster"
		marketplaceconfig.Spec.License.Accept = ptr.Bool(true)
		marketplaceconfig.Status.Conditions.SetCondition(marketplacev1alpha1.ConditionChildRRS3MigrationComplete)
		Expect(k8sClient.Create(context.TODO(), marketplaceconfig.DeepCopy())).Should(Succeed(), "create marketplaceconfig")

	})

	AfterEach(func() {
		rd := &marketplacev1alpha1.RazeeDeployment{}
		err := k8sClient.Get(context.TODO(), types.NamespacedName{
			Name:      name,
			Namespace: operatorNamespace,
		}, rd)
		if !k8serrors.IsNotFound(err) {
			Expect(k8sClient.Delete(context.TODO(), rd)).Should(Succeed())
		}

		operatorSecret := &corev1.Secret{}
		err = k8sClient.Get(context.TODO(), types.NamespacedName{
			Name:      "rhm-operator-secret",
			Namespace: operatorNamespace,
		}, operatorSecret)
		if !k8serrors.IsNotFound(err) {
			Expect(k8sClient.Delete(context.TODO(), operatorSecret)).Should(Succeed())
		}

		marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
		err = k8sClient.Get(context.TODO(), types.NamespacedName{
			Name:      "marketplaceconfig",
			Namespace: operatorNamespace,
		}, marketplaceConfig)
		if !k8serrors.IsNotFound(err) {
			Expect(k8sClient.Delete(context.TODO(), marketplaceConfig)).Should(Succeed())
		}

		cmNames := []string{utils.WATCH_KEEPER_NON_NAMESPACED_NAME, utils.WATCH_KEEPER_LIMITPOLL_NAME, utils.WATCH_KEEPER_CONFIG_NAME, utils.RAZEE_CLUSTER_METADATA_NAME}
		for _, name := range cmNames {
			configMap := &corev1.ConfigMap{}
			err = k8sClient.Get(context.TODO(), types.NamespacedName{
				Name:      name,
				Namespace: operatorNamespace,
			}, configMap)
			if !k8serrors.IsNotFound(err) {
				Expect(k8sClient.Delete(context.TODO(), configMap)).Should(Succeed())
			}
		}

		secretNames := []string{utils.WATCH_KEEPER_SECRET_NAME, utils.RHM_OPERATOR_SECRET_NAME, utils.COS_READER_KEY_NAME}
		for _, name := range secretNames {
			secret := &corev1.Secret{}
			err = k8sClient.Get(context.TODO(), types.NamespacedName{
				Name:      name,
				Namespace: operatorNamespace,
			}, secret)
			if !k8serrors.IsNotFound(err) {
				Expect(k8sClient.Delete(context.TODO(), secret)).Should(Succeed())
			}
		}

		catalogSourceNames := []string{utils.IBM_CATALOGSRC_NAME, utils.OPENCLOUD_CATALOGSRC_NAME}
		for _, name := range catalogSourceNames {
			catalogSource := &operatorsv1alpha1.CatalogSource{}
			err = k8sClient.Get(context.TODO(), types.NamespacedName{
				Name:      name,
				Namespace: utils.OPERATOR_MKTPLACE_NS,
			}, catalogSource)
			if !k8serrors.IsNotFound(err) {
				Expect(k8sClient.Delete(context.TODO(), catalogSource)).Should(Succeed())
			}
		}
	})

	It("find all rerequisite objects", func() {
		Expect(k8sClient.Create(context.TODO(), razeeDeployment.DeepCopy())).Should(Succeed(), "create razeedeployment")
		Expect(k8sClient.Create(context.TODO(), secret.DeepCopy())).Should(Succeed(), "create secret")
		Expect(k8sClient.Create(context.TODO(), console.DeepCopy())).Should(Succeed(), "create console")

		Eventually(func() bool {
			ConfigMapList := &corev1.ConfigMapList{}
			k8sClient.List(context.TODO(), ConfigMapList)

			var configMapNames []string
			for _, cm := range ConfigMapList.Items {
				configMapNames = append(configMapNames, cm.Name)
			}

			return utils.Contains(configMapNames, utils.WATCH_KEEPER_NON_NAMESPACED_NAME) &&
				utils.Contains(configMapNames, utils.WATCH_KEEPER_LIMITPOLL_NAME) &&
				utils.Contains(configMapNames, utils.WATCH_KEEPER_CONFIG_NAME) &&
				utils.Contains(configMapNames, utils.RAZEE_CLUSTER_METADATA_NAME)
		}, timeout, interval).Should(BeTrue())

		Eventually(func() bool {
			secretList := &corev1.SecretList{}
			k8sClient.List(context.TODO(), secretList)

			var secretNames []string
			for _, secret := range secretList.Items {
				secretNames = append(secretNames, secret.Name)
			}

			return utils.Contains(secretNames, utils.WATCH_KEEPER_SECRET_NAME) &&
				utils.Contains(secretNames, utils.RHM_OPERATOR_SECRET_NAME) &&
				utils.Contains(secretNames, utils.COS_READER_KEY_NAME)
		}, timeout, interval).Should(BeTrue())

		Eventually(func() bool {
			catalogSourceList := &operatorsv1alpha1.CatalogSourceList{}
			k8sClient.List(context.TODO(), catalogSourceList)

			var catalogSourceNames []string
			for _, catalogSource := range catalogSourceList.Items {
				catalogSourceNames = append(catalogSourceNames, catalogSource.Name)
			}

			return utils.Contains(catalogSourceNames, utils.IBM_CATALOGSRC_NAME) &&
				utils.Contains(catalogSourceNames, utils.OPENCLOUD_CATALOGSRC_NAME)
		}, timeout, interval).Should(BeTrue())
	})

	It("no secret", func() {
		Expect(k8sClient.Create(context.TODO(), razeeDeployment.DeepCopy())).Should(Succeed(), "create razeedeployment")

		Eventually(func() bool {
			ConfigMapList := &corev1.ConfigMapList{}
			k8sClient.List(context.TODO(), ConfigMapList)

			var configMapNames []string
			for _, cm := range ConfigMapList.Items {
				configMapNames = append(configMapNames, cm.Name)
			}

			utils.PrettyPrint(configMapNames)

			return !utils.Contains(configMapNames, utils.WATCH_KEEPER_NON_NAMESPACED_NAME) &&
				!utils.Contains(configMapNames, utils.WATCH_KEEPER_LIMITPOLL_NAME) &&
				!utils.Contains(configMapNames, utils.WATCH_KEEPER_CONFIG_NAME) &&
				!utils.Contains(configMapNames, utils.RAZEE_CLUSTER_METADATA_NAME)
		}, timeout, interval).Should(BeTrue())

		Eventually(func() bool {
			secretList := &corev1.SecretList{}
			k8sClient.List(context.TODO(), secretList)

			var secretNames []string
			for _, secret := range secretList.Items {
				secretNames = append(secretNames, secret.Name)
			}

			utils.PrettyPrint(secretNames)

			return !utils.Contains(secretNames, utils.WATCH_KEEPER_SECRET_NAME) &&
				!utils.Contains(secretNames, utils.RHM_OPERATOR_SECRET_NAME) &&
				!utils.Contains(secretNames, utils.COS_READER_KEY_NAME)
		}, timeout, interval).Should(BeTrue())

		Eventually(func() bool {
			catalogSourceList := &operatorsv1alpha1.CatalogSourceList{}
			k8sClient.List(context.TODO(), catalogSourceList)

			var catalogSourceNames []string
			for _, catalogSource := range catalogSourceList.Items {
				catalogSourceNames = append(catalogSourceNames, catalogSource.Name)
			}

			utils.PrettyPrint(catalogSourceNames)

			return !utils.Contains(catalogSourceNames, utils.IBM_CATALOGSRC_NAME) &&
				!utils.Contains(catalogSourceNames, utils.OPENCLOUD_CATALOGSRC_NAME)
		}, timeout, interval).Should(BeTrue())
	})
	// 	t := GinkgoT()
	// 	reconcilerTest := NewReconcilerTest(setup, &razeeDeployment, &namespObj)
	// 	reconcilerTest.TestAll(t,
	// 		ReconcileStep(opts,
	// 			ReconcileWithExpectedResults(
	// 				ReconcileResult{}),
	// 		))
	// })

	It("bad name", func() {
		razeeDeploymentLocalDeployment := razeeDeployment.DeepCopy()
		razeeDeploymentLocalDeployment.Name = "foo"
		Expect(k8sClient.Create(context.TODO(), razeeDeploymentLocalDeployment)).Should(Succeed(), "create razeedeployment")

		Eventually(func() bool {
			ConfigMapList := &corev1.ConfigMapList{}
			k8sClient.List(context.TODO(), ConfigMapList)

			var configMapNames []string
			for _, cm := range ConfigMapList.Items {
				configMapNames = append(configMapNames, cm.Name)
			}
			utils.PrettyPrint(configMapNames)
			return !utils.Contains(configMapNames, utils.WATCH_KEEPER_NON_NAMESPACED_NAME) &&
				!utils.Contains(configMapNames, utils.WATCH_KEEPER_LIMITPOLL_NAME) &&
				!utils.Contains(configMapNames, utils.WATCH_KEEPER_CONFIG_NAME) &&
				!utils.Contains(configMapNames, utils.RAZEE_CLUSTER_METADATA_NAME)
		}, timeout, interval).Should(BeTrue())

		Eventually(func() bool {
			secretList := &corev1.SecretList{}
			k8sClient.List(context.TODO(), secretList)

			var secretNames []string
			for _, secret := range secretList.Items {
				secretNames = append(secretNames, secret.Name)
			}

			utils.PrettyPrint(secretNames)
			return !utils.Contains(secretNames, utils.WATCH_KEEPER_SECRET_NAME) &&
				!utils.Contains(secretNames, utils.RHM_OPERATOR_SECRET_NAME) &&
				!utils.Contains(secretNames, utils.COS_READER_KEY_NAME)
		}, timeout, interval).Should(BeTrue())

		Eventually(func() bool {
			catalogSourceList := &operatorsv1alpha1.CatalogSourceList{}
			k8sClient.List(context.TODO(), catalogSourceList)

			var catalogSourceNames []string
			for _, catalogSource := range catalogSourceList.Items {
				utils.PrettyPrint(catalogSource)
				catalogSourceNames = append(catalogSourceNames, catalogSource.Name)
			}

			return !utils.Contains(catalogSourceNames, utils.IBM_CATALOGSRC_NAME) &&
				!utils.Contains(catalogSourceNames, utils.OPENCLOUD_CATALOGSRC_NAME)
		}, timeout, interval).Should(BeTrue())
	})

	It("full uninstall", func() {
		Expect(k8sClient.Create(context.TODO(), razeeDeploymentDeletion.DeepCopy())).Should(Succeed(), "create razeedeployment")
		Expect(k8sClient.Create(context.TODO(), secret.DeepCopy())).Should(Succeed(), "create rhm-operator-secret")

		Eventually(func() bool {
			rd := &marketplacev1alpha1.RazeeDeployment{}
			Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
				Name:      name,
				Namespace: operatorNamespace,
			}, rd)).Should(Succeed(), "get razeedeployment")
			err := k8sClient.Delete(context.TODO(), rd)
			return err == nil
		})

		Eventually(func() bool {
			rd := &corev1.Secret{}
			Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
				Name:      secretName,
				Namespace: operatorNamespace,
			}, rd)).Should(Succeed(), "get razeedeployment")
			err := k8sClient.Delete(context.TODO(), rd)
			return err == nil
		})

		// Eventually(func() []marketplacev1alpha1.RazeeDeployment {
		// 	rd := &marketplacev1alpha1.RazeeDeployment{}
		// 	Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
		// 		Name:      name,
		// 		Namespace: operatorNamespace,
		// 	}, rd)).Should(Succeed(), "get razeedeployment")
		// 	k8sClient.Delete(context.TODO(), rd)

		// 	list := &marketplacev1alpha1.RazeeDeploymentList{}
		// 	k8sClient.List(context.TODO(), list, client.InNamespace(operatorNamespace))

		// 	return list.Items
		// }, timeout, interval).Should(HaveLen(0), "razee deployment cr should be cleaned up")

		Eventually(func() []razeev1alpha2.RemoteResource {
			list := &razeev1alpha2.RemoteResourceList{}
			k8sClient.List(context.TODO(), list, client.InNamespace(operatorNamespace))

			return list.Items
		}, timeout, interval).Should(HaveLen(0), "system RemoteResource should be deleted")

		Eventually(func() []corev1.ConfigMap {
			list := &corev1.ConfigMapList{}
			k8sClient.List(context.TODO(), list, client.InNamespace(operatorNamespace))

			return list.Items
		}, timeout, interval).Should(HaveLen(0), "system ConfigMaps should be deleted")

		Eventually(func() []corev1.Secret {
			list := &corev1.SecretList{}
			k8sClient.List(context.TODO(), list, client.InNamespace(operatorNamespace))

			return list.Items
		}, timeout, interval).Should(HaveLen(0), "system Secrets should be deleted")

		Eventually(func() []appsv1.Deployment {
			list := &appsv1.DeploymentList{}
			k8sClient.List(context.TODO(), list, client.InNamespace(operatorNamespace))

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
