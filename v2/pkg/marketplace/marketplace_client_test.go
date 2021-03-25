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
	"crypto/tls"
	ioutil "io/ioutil"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
)

const timeout = time.Second * 100
const interval = time.Second * 3

var _ = Describe("Marketplace Config Status", func() {
	var (
		marketplaceClientAccount *MarketplaceClientAccount
		mclient                  *MarketplaceClient
		registrationStatus       *RegistrationStatusOutput
		server                   *ghttp.Server
		statusCode               int
		body                     []byte
		path                     string
		err                      error
		mbuilder                 *MarketplaceClientBuilder
	)

	BeforeEach(func() {
		// start a test http server
		server = ghttp.NewTLSServer()
		server.SetAllowUnhandledRequests(true)

		addr := "https://" + server.Addr() + path

		cfg := &config.OperatorConfig{
			Marketplace: config.Marketplace{
				URL:            addr,
				InsecureClient: false,
			},
		}

		token := "Bearer token"
		tokenClaims := &MarketplaceClaims{
			Env: "",
		}

		mbuilder = NewMarketplaceClientBuilder(cfg).SetTLSConfig(&tls.Config{
			RootCAs:            server.HTTPTestServer.TLS.RootCAs,
			InsecureSkipVerify: true,
		})

		mclient, err = mbuilder.NewMarketplaceClient(token, tokenClaims)
		Expect(err).To(Succeed())

		Expect(mclient.endpoint).ToNot(BeNil())

		marketplaceClientAccount = &MarketplaceClientAccount{
			AccountId:   "accountid",
			ClusterUuid: "test",
		}
	})

	AfterEach(func() {
		server.Close()
	})

	Context("Marketplace Pull Secret without any error", func() {
		BeforeEach(func() {
			statusCode = 200
			path = "/" + PullSecretEndpoint
			body, err := ioutil.ReadFile("../../tests/mockresponses/marketplace-pull-secret.yaml")
			if err != nil {
				panic(err)
			}

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", path),
					ghttp.RespondWithPtr(&statusCode, &body),
				),
			)
		})
		It("should retrieve rhm-operator-secret ", func() {
			data, err := mclient.GetMarketplaceSecret()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(data).Should(ContainSubstring("rhm-operator-secret"))
		})
	})

	Context("token", func() {
		It("should have env var", func() {
			Skip("can't keep test due to secret")
			token := ``
			rhmAccount, err := GetJWTTokenClaim(token)
			Expect(err).ToNot(HaveOccurred())
			Expect(rhmAccount.AccountID).To(Equal("5e2f551de3957e0013215b2d"))
		})
	})

	Context("Cluster Registration Status is INSTALLED", func() {
		BeforeEach(func() {
			statusCode = 200
			path = "/" + RegistrationEndpoint

			body, _ = ioutil.ReadFile("../../tests/mockresponses/registration-response.json")
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", path, "accountId=accountid&uuid=test"),
					ghttp.RespondWithPtr(&statusCode, &body),
				))
		})
		It("Expect true value for registration status", func() {
			registrationStatusOutput, err := mclient.RegistrationStatus(marketplaceClientAccount)
			Expect(err).ToNot(HaveOccurred())
			Expect(registrationStatusOutput.RegistrationStatus).To(Equal("INSTALLED"))
		})
	})

	Context("Cluster Registration Status is blank", func() {
		BeforeEach(func() {
			statusCode = 200
			path = "/" + RegistrationEndpoint
			body = []byte("[]")
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", path),
					ghttp.RespondWithPtr(&statusCode, &body),
				))

		})
		It("Expect true value for registration status", func() {
			registrationStatusOutput, err := mclient.RegistrationStatus(marketplaceClientAccount)
			Expect(err).ToNot(HaveOccurred())
			Expect(registrationStatusOutput.RegistrationStatus).To(Equal("UNREGISTERED"))

		})
	})

	Context("Marketplace Config Status with http status code 200", func() {
		BeforeEach(func() {
			registrationStatus = &RegistrationStatusOutput{
				StatusCode:         200,
				RegistrationStatus: "INSTALLED",
			}

		})
		It("Expect 200 status and status as registered", func() {
			statusConditions := registrationStatus.TransformConfigStatus()
			Expect(statusConditions.IsTrueFor(marketplacev1alpha1.ConditionRegistered)).To(BeTrue())
			reason := statusConditions.GetCondition(marketplacev1alpha1.ConditionRegistered).Reason
			Expect(reason).To(Equal(marketplacev1alpha1.ReasonRegistrationSuccess))
		})
	})

	Context("Marketplace Config Status with http status code 500", func() {
		BeforeEach(func() {
			registrationStatus = &RegistrationStatusOutput{
				StatusCode:         500,
				RegistrationStatus: "error",
			}
		})
		It("Expect 500 status and status as registered", func() {
			statusConditions := registrationStatus.TransformConfigStatus()
			Expect(statusConditions.IsFalseFor(marketplacev1alpha1.ConditionRegistered)).To(BeTrue())
			reason := statusConditions.GetCondition(marketplacev1alpha1.ConditionRegistered).Reason
			Expect(reason).To(Equal(marketplacev1alpha1.ReasonRegistrationError))

		})
	})

	Context("Marketplace Config Status with http status code 408", func() {
		BeforeEach(func() {
			registrationStatus = &RegistrationStatusOutput{
				StatusCode:         408,
				RegistrationStatus: "error",
			}
		})
		It("Expect 408 status and status as registered", func() {
			statusConditions := registrationStatus.TransformConfigStatus()
			Expect(statusConditions.IsFalseFor(marketplacev1alpha1.ConditionRegistered)).To(BeTrue())
			reason := statusConditions.GetCondition(marketplacev1alpha1.ConditionRegistered).Reason
			Expect(reason).To(Equal(marketplacev1alpha1.ReasonRegistrationError))
		})
	})

	Context("Marketplace Config Status with http status code 400", func() {
		BeforeEach(func() {
			registrationStatus = &RegistrationStatusOutput{
				StatusCode:         400,
				RegistrationStatus: "error",
			}
		})
		It("Expect 400 status and status as registered", func() {
			statusConditions := registrationStatus.TransformConfigStatus()
			Expect(statusConditions.IsFalseFor(marketplacev1alpha1.ConditionRegistered)).To(BeTrue())

			reason := statusConditions.GetCondition(marketplacev1alpha1.ConditionRegistered).Reason
			Expect(reason).To(Equal(marketplacev1alpha1.ReasonRegistrationError))
		})
	})
	Context("Marketplace Config Status with http status code 300", func() {
		BeforeEach(func() {
			registrationStatus = &RegistrationStatusOutput{
				StatusCode:         300,
				RegistrationStatus: "error",
			}
		})
		It("Expect 300 status and status as registered", func() {
			statusConditions := registrationStatus.TransformConfigStatus()
			Expect(statusConditions.IsFalseFor(marketplacev1alpha1.ConditionRegistered)).To(BeTrue())
		})
	})
	/*
		Context("Marketplace Pull Secret without any error", func() {
				BeforeEach(func() {
					statusCode = 200
					path = "/provisioning/v1/rhm-operator/rhm-operator-secret"
					body, err := ioutil.ReadFile("../../tests/mockresponses/marketplace-pull-secret.yaml")
					if err != nil {
						panic(err)
					}
					addr = "http://" + server.Addr() + path
					marketplaceClientConfig = &MarketplaceClientConfig{
						Url:   addr,
						Token: "Bearer token",
					}

					server.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", path),
							ghttp.RespondWithPtr(&statusCode, &body),
						))

				})
				It("Expect rhm-operator-secret ", func() {
					data, err := GetMarketPlaceSecret(marketplaceClientConfig)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(data).Should(ContainSubstring("rhm-operator-secret"))

				})
			})*/
})
