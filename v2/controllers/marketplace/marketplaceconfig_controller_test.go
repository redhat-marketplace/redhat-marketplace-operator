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

	"github.com/golang-jwt/jwt/v5"
	"github.com/gotidy/ptr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var (
	err        error
	customerID string = "accountid"

	secret      *corev1.Secret
	tokenString string
	server      *ghttp.Server
)

var _ = Describe("Testing MarketplaceConfig controller", func() {

	marketplaceconfig := utils.BuildMarketplaceConfigCR(operatorNamespace, customerID)
	marketplaceconfig.Spec.ClusterUUID = "test"
	marketplaceconfig.Spec.IsDisconnected = ptr.Bool(true)
	marketplaceconfig.Spec.ClusterName = "test-cluster"
	marketplaceconfig.Spec.License.Accept = ptr.Bool(true)

	marketplaceconfigConnected := utils.BuildMarketplaceConfigCR(operatorNamespace, customerID)
	marketplaceconfigConnected.Spec.ClusterUUID = "test"
	marketplaceconfigConnected.Spec.ClusterName = "test-cluster-connected"
	marketplaceconfigConnected.Spec.InstallIBMCatalogSource = ptr.Bool(true)
	marketplaceconfigConnected.Spec.License.Accept = ptr.Bool(true)

	BeforeEach(func() {
		expireTime := time.Now().Add(1500 * time.Second)
		// setup redhat-marketplace-pull-secret
		tokenClaims := utils.MarketplaceClaims{
			AccountID: "foo",
			APIKey:    "test",
			Env:       "test",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: &jwt.NumericDate{
					Time: expireTime,
				},
				Issuer: "test",
			},
		}

		signingKey := []byte("AllYourBase")
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, tokenClaims)

		Eventually(func() string {
			tokenString, err = token.SignedString(signingKey)
			if err != nil {
				panic(err)
			}
			return tokenString
		}, timeout, interval).ShouldNot(BeEmpty())

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      utils.RHMPullSecretName,
				Namespace: operatorNamespace,
			},
			Data: map[string][]byte{
				utils.RHMPullSecretKey: []byte(tokenString),
			},
		}

		// setup mock backend server
		server = ghttp.NewTLSServer()
		server.SetAllowUnhandledRequests(true)
	})

	AfterEach(func() {

		Cleanup()

		server.Close()
	})

	It("marketplace config controller in disconnected mode", func() {

		// create required resources
		Eventually(func() bool {
			var failed bool
			err := k8sClient.Create(context.TODO(), marketplaceconfig.DeepCopy())
			if err != nil {
				failed = true
			}

			return failed
		}, timeout, interval).ShouldNot(BeTrue())

		Eventually(func() bool {
			var failed bool
			err := k8sClient.Create(context.TODO(), secret.DeepCopy())
			if err != nil {
				failed = true
			}

			return failed
		}, timeout, interval).ShouldNot(BeTrue())

		// fetch created resources
		rd := &marketplacev1alpha1.RazeeDeployment{}
		Eventually(func() bool {
			var notFound bool
			err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: utils.RAZEE_NAME, Namespace: operatorNamespace}, rd)
			if k8serrors.IsNotFound(err) {
				notFound = true
			} else if rd.Spec.Features == nil { // wait for init
				notFound = true
			}

			return notFound
		}, timeout, interval).ShouldNot(BeTrue())

		mb := &marketplacev1alpha1.MeterBase{}
		Eventually(func() bool {
			var notFound bool
			err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: utils.METERBASE_NAME, Namespace: operatorNamespace}, mb)
			if k8serrors.IsNotFound(err) {
				notFound = true
			}

			return notFound
		}, timeout, interval).ShouldNot(BeTrue())

		mc := &marketplacev1alpha1.MarketplaceConfig{}
		Eventually(func() bool {
			var notFound bool
			err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: utils.MARKETPLACECONFIG_NAME, Namespace: operatorNamespace}, mc)
			if k8serrors.IsNotFound(err) {
				notFound = true
			} else if mc.Spec.Features == nil { // wait for init
				notFound = true
			}

			return notFound
		}, timeout, interval).ShouldNot(BeTrue())

		Eventually(mc.Status.Conditions.GetCondition(marketplacev1alpha1.ConditionIsDisconnected).Message, timeout, interval).Should(Equal("Detected disconnected environment"))
		Expect(*mc.Spec.Features.Deployment).Should(BeFalse())
		Expect(*mc.Spec.Features.Registration).Should(BeTrue())
		Expect(*mc.Spec.Features.EnableMeterDefinitionCatalogServer).Should(BeFalse())
		Eventually(mc.Status.Conditions.GetCondition(marketplacev1alpha1.ConditionComplete), timeout, interval).ShouldNot(BeNil())
		Eventually(mc.Status.Conditions.GetCondition(marketplacev1alpha1.ConditionComplete).Message, timeout, interval).Should(Equal("Finished Installing necessary components"))

		Expect(mb.Spec.Enabled).Should(BeTrue())
		Expect(mb.Spec.MeterdefinitionCatalogServerConfig.DeployMeterDefinitionCatalogServer).Should(BeFalse())
		Expect(mb.Spec.MeterdefinitionCatalogServerConfig.SyncCommunityMeterDefinitions).Should(BeFalse())
		Expect(mb.Spec.MeterdefinitionCatalogServerConfig.SyncSystemMeterDefinitions).Should(BeFalse())

		Expect(*rd.Spec.Features.Deployment).Should(BeFalse())
		Expect(*rd.Spec.Features.Registration).Should(BeTrue())
		Expect(*rd.Spec.Features.EnableMeterDefinitionCatalogServer).Should(BeFalse())
		Eventually(rd.Spec.ClusterDisplayName, timeout, interval).Should(Equal("test-cluster"))
	})

	It("marketplace config controller in connected mode", func() {

		Eventually(func() bool {
			var failed bool
			err := k8sClient.Create(context.TODO(), marketplaceconfigConnected.DeepCopy())
			if err != nil {
				failed = true
			}

			return failed
		}, timeout, interval).ShouldNot(BeTrue())

		Eventually(func() bool {
			var failed bool
			err := k8sClient.Create(context.TODO(), secret.DeepCopy())
			if err != nil {
				failed = true
			}

			return failed
		}, timeout, interval).ShouldNot(BeTrue())

		// fetch created resources
		rd := &marketplacev1alpha1.RazeeDeployment{}
		Eventually(func() bool {
			var notFound bool
			err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: utils.RAZEE_NAME, Namespace: operatorNamespace}, rd)
			if k8serrors.IsNotFound(err) {
				notFound = true
			} else if rd.Spec.Features == nil { // wait for init
				notFound = true
			}

			return notFound
		}, timeout, interval).ShouldNot(BeTrue())

		mb := &marketplacev1alpha1.MeterBase{}
		Eventually(func() bool {
			var notFound bool
			err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: utils.METERBASE_NAME, Namespace: operatorNamespace}, mb)
			if k8serrors.IsNotFound(err) {
				notFound = true
			}

			return notFound
		}, timeout, interval).ShouldNot(BeTrue())

		mc := &marketplacev1alpha1.MarketplaceConfig{}
		Eventually(func() bool {
			var notFound bool
			err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: utils.MARKETPLACECONFIG_NAME, Namespace: operatorNamespace}, mc)
			if k8serrors.IsNotFound(err) {
				notFound = true
			} else if mc.Spec.Features == nil { // wait for init
				notFound = true
			}

			return notFound
		}, timeout, interval).ShouldNot(BeTrue())

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

		Expect(*mc.Spec.Features.Deployment).Should(BeFalse())
		Expect(*mc.Spec.Features.Registration).Should(BeTrue())
		Expect(*mc.Spec.Features.EnableMeterDefinitionCatalogServer).Should(BeFalse())
		Expect(mc.Status.Conditions.GetCondition(marketplacev1alpha1.ConditionComplete).Message).Should(Equal("Finished Installing necessary components"))

		Expect(mb.Spec.Enabled).Should(BeTrue())
		Expect(mb.Spec.MeterdefinitionCatalogServerConfig.DeployMeterDefinitionCatalogServer).Should(BeFalse())
		Expect(mb.Spec.MeterdefinitionCatalogServerConfig.SyncCommunityMeterDefinitions).Should(BeFalse())
		Expect(mb.Spec.MeterdefinitionCatalogServerConfig.SyncSystemMeterDefinitions).Should(BeFalse())

		Expect(*rd.Spec.Features.Deployment).Should(BeFalse())
		Expect(*rd.Spec.Features.Registration).Should(BeTrue())
		Expect(*rd.Spec.Features.EnableMeterDefinitionCatalogServer).Should(BeFalse())
		Expect(rd.Spec.ClusterDisplayName).Should(Equal("test-cluster-connected"))
	})
})
