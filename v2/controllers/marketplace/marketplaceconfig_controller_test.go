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
	"io/ioutil"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gotidy/ptr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	opsrcApi "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/marketplace"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/tests/rectest"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const timeout = time.Second * 20
const interval = time.Second * 3

var _ = Describe("Testing with Ginkgo", func() {
	var (
		server        *ghttp.Server
		statusCode    int
		body          []byte
		path          string
		err           error
		name                 = utils.MARKETPLACECONFIG_NAME
		namespace            = "redhat-marketplace-operator"
		customerID    string = "accountid"
		razeeName            = "rhm-marketplaceconfig-razeedeployment"
		meterBaseName        = "rhm-marketplaceconfig-meterbase"
		req                  = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		}

		opts = []StepOption{
			WithRequest(req),
		}

		features = &common.Features{
			Deployment:                         ptr.Bool(true),
			EnableMeterDefinitionCatalogServer: ptr.Bool(true),
		}

		deployedNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}

		secret      *corev1.Secret
		tokenString string

		marketplaceconfig *marketplacev1alpha1.MarketplaceConfig
		razeedeployment   *marketplacev1alpha1.RazeeDeployment
		meterbase         *marketplacev1alpha1.MeterBase
		cfg               *config.OperatorConfig
	)

	BeforeEach(func() {
		marketplaceconfig = utils.BuildMarketplaceConfigCR(namespace, customerID)
		marketplaceconfig.Spec.ClusterUUID = "test"
		marketplaceconfig.Spec.Features = features
		razeedeployment = utils.BuildRazeeCr(namespace, marketplaceconfig.Spec.ClusterUUID, marketplaceconfig.Spec.DeploySecretName, features)
		meterbase = utils.BuildMeterBaseCr(namespace, *marketplaceconfig.Spec.Features.EnableMeterDefinitionCatalogServer)
		tokenClaims := marketplace.MarketplaceClaims{
			AccountID: "foo",
			APIKey:    "test",
			Env:       "test",
			StandardClaims: jwt.StandardClaims{
				ExpiresAt: 15000,
				Issuer:    "test",
			},
		}

		signingKey := []byte("AllYourBase")
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, tokenClaims)

		Eventually(func() string {
			// Create the token
			tokenString, err = token.SignedString(signingKey)
			if err != nil {
				panic(err)
			}

			return tokenString
		}, timeout, interval).ShouldNot(BeEmpty())
		// start a test http server
		server = ghttp.NewTLSServer()
		server.SetAllowUnhandledRequests(true)
		addr := "https://" + server.Addr() + path
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      utils.RHMPullSecretName,
				Namespace: namespace,
			},
			Data: map[string][]byte{
				utils.RHMPullSecretKey: []byte(tokenString),
			},
		}
		cfg = &config.OperatorConfig{
			DeployedNamespace: namespace,
			Marketplace: config.Marketplace{
				URL:            addr,
				InsecureClient: true,
			},
		}

		statusCode = 200
		path = "/" + marketplace.RegistrationEndpoint
		body, err = ioutil.ReadFile("../../tests/mockresponses/registration-response.json")
		if err != nil {
			panic(err)
		}

		server.RouteToHandler(
			"GET", path, ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", path, "accountId=accountid&uuid=test"),
				ghttp.RespondWithPtr(&statusCode, &body),
			))
	})

	AfterEach(func() {
		server.Close()
	})

	It("marketplace config controller", func() {
		var testCleanInstall = func(t GinkgoTInterface) {
			var setup = func(r *ReconcilerTest) error {
				var log = logf.Log.WithName("mockcontroller")
				s := provideScheme()
				_ = opsrcApi.AddToScheme(s)
				_ = operatorsv1alpha1.AddToScheme(s)
				s.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, marketplaceconfig)
				s.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, razeedeployment)
				s.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, meterbase)

				r.Client = fake.NewFakeClientWithScheme(s, r.GetGetObjects()...)
				r.Reconciler = &MarketplaceConfigReconciler{
					Client: r.Client,
					Scheme: s,
					Log:    log,
					cc:     reconcileutils.NewLoglessClientCommand(r.Client, s),
					cfg:    cfg,
				}
				return nil
			}
			marketplaceconfig.Spec.EnableMetering = ptr.Bool(true)
			marketplaceconfig.Spec.InstallIBMCatalogSource = ptr.Bool(true)

			reconcilerTest := NewReconcilerTest(setup, marketplaceconfig, deployedNamespace, secret)
			reconcilerTest.TestAll(t,
				ReconcileStep(opts, ReconcileWithUntilDone(true)),
				GetStep(opts,
					GetWithNamespacedName(razeeName, namespace),
					GetWithObj(&marketplacev1alpha1.RazeeDeployment{}),
				),
				GetStep(opts,
					GetWithNamespacedName(meterBaseName, namespace),
					GetWithObj(&marketplacev1alpha1.MeterBase{}),
				),
				GetStep(opts,
					GetWithNamespacedName(utils.IBM_CATALOGSRC_NAME, utils.OPERATOR_MKTPLACE_NS),
					GetWithObj(&operatorsv1alpha1.CatalogSource{}),
				),
				GetStep(opts,
					GetWithNamespacedName(utils.OPENCLOUD_CATALOGSRC_NAME, utils.OPERATOR_MKTPLACE_NS),
					GetWithObj(&operatorsv1alpha1.CatalogSource{}),
				),
			)
		}

		defaultFeatures := []string{"razee", "meterbase"}
		viper.Set("features", defaultFeatures)
		viper.Set("IBMCatalogSource", true)
		testCleanInstall(GinkgoT())

	})
})
