package marketplace

import (
	ioutil "io/ioutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
)

var _ = Describe("Marketplace Config Status", func() {
	var (
		marketplaceClientConfig  *MarketplaceClientConfig
		marketplaceClientAccount *MarketplaceClientAccount
		newMarketPlaceClient     *MarketplaceClient
		registrationStatus       *RegistrationStatusOutput
		server                   *ghttp.Server
		statusCode               int
		body                     []byte
		path                     string
		addr                     string
	)

	BeforeEach(func() {
		// start a test http server
		server = ghttp.NewServer()

	})
	AfterEach(func() {
		server.Close()
	})
	Context("Marketplace Pull Secret without any error", func() {
		BeforeEach(func() {
			statusCode = 200
			path = "/provisioning/v1/rhm-operator/rhm-operator-secret"
			body, err := ioutil.ReadFile("../../test/mockresponses/marketplace-pull-secret.yaml")
			if err != nil {
				panic(err)
			}
			addr = "http://" + server.Addr() + path
			marketplaceClientConfig = &MarketplaceClientConfig{
				Url:   addr,
				Token: "Bearer token",
			}
			newMarketPlaceClient, _ = NewMarketplaceClient(marketplaceClientConfig)
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", path),
					ghttp.RespondWithPtr(&statusCode, &body),
				))

		})
		It("Expect rhm-operator-secret ", func() {
			data, err := newMarketPlaceClient.GetMarketPlaceSecret()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(data).Should(ContainSubstring("rhm-operator-secret"))

		})
	})
	Context("Cluster Registration Status is INSTALLED", func() {
		BeforeEach(func() {
			statusCode = 200
			path = "/provisioning/v1/registered-clusters"
			body, _ = ioutil.ReadFile("../../test/mockresponses/registration-response.json")
			addr = "http://" + server.Addr() + path
			marketplaceClientConfig = &MarketplaceClientConfig{
				Url:   addr,
				Token: "Bearer token",
			}
			marketplaceClientAccount = &MarketplaceClientAccount{
				AccountId:   "accountid",
				ClusterUuid: "clusterid",
			}
			newMarketPlaceClient, _ = NewMarketplaceClient(marketplaceClientConfig)
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", path),
					ghttp.RespondWithPtr(&statusCode, &body),
				))

		})
		It("Expect true value for registration status", func() {
			registrationStatusOutput := newMarketPlaceClient.RegistrationStatus(marketplaceClientAccount)
			Expect(registrationStatusOutput.RegistrationStatus).To(Equal("INSTALLED"))

		})
	})

	Context("Cluster Registration Status is blank", func() {
		BeforeEach(func() {
			statusCode = 200
			path = "/provisioning/v1/registered-clusters"
			body = []byte("[]")
			addr = "http://" + server.Addr() + path
			marketplaceClientConfig = &MarketplaceClientConfig{
				Url:   addr,
				Token: "Bearer token",
			}
			marketplaceClientAccount = &MarketplaceClientAccount{
				AccountId:   "accountid",
				ClusterUuid: "clusterid",
			}
			newMarketPlaceClient, _ = NewMarketplaceClient(marketplaceClientConfig)
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", path),
					ghttp.RespondWithPtr(&statusCode, &body),
				))

		})
		It("Expect true value for registration status", func() {
			registrationStatusOutput := newMarketPlaceClient.RegistrationStatus(marketplaceClientAccount)
			Expect(registrationStatusOutput.RegistrationStatus).To(Equal("NotRegistered"))

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
			statusConditions := TransformConfigStatus(*registrationStatus)
			Expect(statusConditions.Reason).To(Equal(marketplacev1alpha1.ReasonRegistrationStatus))

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
			statusConditions := TransformConfigStatus(*registrationStatus)
			Expect(statusConditions.Reason).To(Equal(marketplacev1alpha1.ReasonServiceUnavailable))

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
			statusConditions := TransformConfigStatus(*registrationStatus)
			Expect(statusConditions.Reason).To(Equal(marketplacev1alpha1.ReasonInternetDisconnected))

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
			statusConditions := TransformConfigStatus(*registrationStatus)
			Expect(statusConditions.Reason).To(Equal(marketplacev1alpha1.ReasonClientError))

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
			statusConditions := TransformConfigStatus(*registrationStatus)
			Expect(statusConditions.Reason).To(Equal(marketplacev1alpha1.ReasonRegistrationError))

		})
	})
	/*
		Context("Marketplace Pull Secret without any error", func() {
				BeforeEach(func() {
					statusCode = 200
					path = "/provisioning/v1/rhm-operator/rhm-operator-secret"
					body, err := ioutil.ReadFile("../../test/mockresponses/marketplace-pull-secret.yaml")
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
