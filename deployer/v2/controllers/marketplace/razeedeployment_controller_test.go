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
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
)

var _ = Describe("Testing with Ginkgo", func() {
	var (
		name            = utils.RAZEE_NAME
		secretName      = utils.RHM_OPERATOR_SECRET_NAME
		razeeDeployment marketplacev1alpha1.RazeeDeployment
		secret          corev1.Secret
	)

	BeforeEach(func() {

		name = utils.RAZEE_NAME
		secretName = utils.RHM_OPERATOR_SECRET_NAME

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

		secret = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      utils.RHM_OPERATOR_SECRET_NAME,
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
			Name:      utils.RHM_OPERATOR_SECRET_NAME,
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

			return !utils.Contains(secretNames, utils.WATCH_KEEPER_SECRET_NAME) &&
				!utils.Contains(secretNames, utils.RHM_OPERATOR_SECRET_NAME) &&
				!utils.Contains(secretNames, utils.COS_READER_KEY_NAME)
		}, timeout, interval).Should(BeTrue())

		// Catalogs are reconciled regardless of secret
		Eventually(func() bool {
			catalogSourceList := &operatorsv1alpha1.CatalogSourceList{}
			k8sClient.List(context.TODO(), catalogSourceList)

			var catalogSourceNames []string
			for _, catalogSource := range catalogSourceList.Items {
				catalogSourceNames = append(catalogSourceNames, catalogSource.Name)
			}

			utils.PrettyPrint(catalogSourceNames)

			return utils.Contains(catalogSourceNames, utils.IBM_CATALOGSRC_NAME) &&
				utils.Contains(catalogSourceNames, utils.OPENCLOUD_CATALOGSRC_NAME)
		}, timeout, interval).Should(BeTrue())
	})

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

			return !utils.Contains(catalogSourceNames, utils.IBM_CATALOGSRC_NAME) &&
				!utils.Contains(catalogSourceNames, utils.OPENCLOUD_CATALOGSRC_NAME)
		}, timeout, interval).Should(BeTrue())
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
