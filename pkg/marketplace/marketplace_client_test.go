package marketplace

import (
	ioutil "io/ioutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/operator-framework/operator-sdk/pkg/status"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
)

var _ = Describe("Marketplace Config Status", func() {
	var (
		marketplaceClientConfig  *MarketplaceClientConfig
		marketplaceClientAccount *MarketplaceClientAccount
		conditions               *status.Conditions
		//marketplaceConfig       *marketplacev1alpha1.MarketplaceConfig
		server     *ghttp.Server
		statusCode int
		body       []byte
		path       string
		addr       string
	)

	BeforeEach(func() {
		// start a test http server
		server = ghttp.NewServer()
		body, _ = ioutil.ReadFile("../../test/mockresponses/registration-response.json")
	})
	AfterEach(func() {
		server.Close()
	})

	Context("Cluster Registration Status is INSTALLED", func() {
		BeforeEach(func() {
			statusCode = 200
			path = "/provisioning/v1/registered-clusters"
			//body, _ = ioutil.ReadFile("../../test/mockresponses/registration-response.json")
			addr = "http://" + server.Addr() + path
			marketplaceClientConfig = &MarketplaceClientConfig{
				Url:   addr,
				Token: "Bearer token",
			}
			marketplaceClientAccount = &MarketplaceClientAccount{
				AccountId:   "accountid",
				ClusterUuid: "clusterid",
			}

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", path),
					ghttp.RespondWithPtr(&statusCode, &body),
				))

		})
		It("Expect true value for registration status", func() {
			registrationStatusOutput, err := ClusterRegistrationStatus(marketplaceClientConfig, marketplaceClientAccount)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(registrationStatusOutput.RegistrationStatus).To(Equal("INSTALLED"))

		})
	})

	Context("Marketplace Config Status with http status code 200", func() {
		BeforeEach(func() {
			statusCode = 200
			path = "/provisioning/v1/registered-clusters"
			//body = []byte("registered")
			addr = "http://" + server.Addr() + path
			marketplaceClientConfig = &MarketplaceClientConfig{
				Url:   addr,
				Token: "Bearer token",
			}
			marketplaceClientAccount = &MarketplaceClientAccount{
				AccountId:   "accountid",
				ClusterUuid: "clusterid",
			}
			//newMarketplaceConfig = &marketplacev1alpha1.MarketplaceConfig{}
			conditions = &status.Conditions{}
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", path),
					ghttp.RespondWithPtr(&statusCode, &body),
				))

		})
		It("Expect 200 status and status as registered", func() {
			statusConditions, err := ClusterRegistrationStatusConditions(marketplaceClientConfig, marketplaceClientAccount, conditions)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(statusConditions.GetCondition(marketplacev1alpha1.ConditionRegistered).Reason).To(Equal(marketplacev1alpha1.ReasonRegistrationStatus))

		})
	})

	Context("Marketplace Config Status with http status code 500", func() {
		BeforeEach(func() {
			statusCode = 500
			path = "/provisioning/v1/registered-clusters"
			//body = []byte("registered")
			addr = "http://" + server.Addr() + path
			marketplaceClientConfig = &MarketplaceClientConfig{
				Url:   addr,
				Token: "Bearer token",
			}
			marketplaceClientAccount = &MarketplaceClientAccount{
				AccountId:   "accountid",
				ClusterUuid: "clusterid",
			}
			//newMarketplaceConfig = &marketplacev1alpha1.MarketplaceConfig{}
			conditions = &status.Conditions{}
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", path),
					ghttp.RespondWithPtr(&statusCode, &body),
				))

		})
		It("Expect 500 status", func() {
			statusConditions, err := ClusterRegistrationStatusConditions(marketplaceClientConfig, marketplaceClientAccount, conditions)
			Expect(err).ShouldNot(HaveOccurred())
			//Expect(updatedMarketplaceConfig.Status.Conditions.GetCondition(marketplacev1alpha1.ConditionRegistrationError)).ShouldNot(BeNil())
			Expect(statusConditions.GetCondition(marketplacev1alpha1.ConditionRegistrationError).Reason).To(Equal(marketplacev1alpha1.ReasonServiceUnavailable))

		})
	})

	Context("Marketplace Config Status with http status code 408", func() {
		BeforeEach(func() {
			statusCode = 408
			path = "/provisioning/v1/registered-clusters"
			//body = []byte("registered")
			addr = "http://" + server.Addr() + path
			marketplaceClientConfig = &MarketplaceClientConfig{
				Url:   addr,
				Token: "Bearer token",
			}
			marketplaceClientAccount = &MarketplaceClientAccount{
				AccountId:   "accountid",
				ClusterUuid: "clusterid",
			}
			//newMarketplaceConfig = &marketplacev1alpha1.MarketplaceConfig{}
			conditions = &status.Conditions{}
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", path),
					ghttp.RespondWithPtr(&statusCode, &body),
				))

		})
		It("Expect 408 status", func() {
			statusConditions, err := ClusterRegistrationStatusConditions(marketplaceClientConfig, marketplaceClientAccount, conditions)
			Expect(err).ShouldNot(HaveOccurred())
			//Expect(updatedMarketplaceConfig.Status.Conditions.GetCondition(marketplacev1alpha1.ConditionRegistrationError)).ShouldNot(BeNil())
			Expect(statusConditions.GetCondition(marketplacev1alpha1.ConditionRegistrationError).Reason).To(Equal(marketplacev1alpha1.ReasonInternetDisconnected))

		})
	})
	Context("Marketplace Config Status with http status code 400", func() {
		BeforeEach(func() {
			statusCode = 400
			path = "/provisioning/v1/registered-clusters"
			//body = []byte("registered")
			addr = "http://" + server.Addr() + path
			marketplaceClientConfig = &MarketplaceClientConfig{
				Url:   addr,
				Token: "Bearer token",
			}
			marketplaceClientAccount = &MarketplaceClientAccount{
				AccountId:   "accountid",
				ClusterUuid: "clusterid",
			}
			//newMarketplaceConfig = &marketplacev1alpha1.MarketplaceConfig{}
			conditions = &status.Conditions{}
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", path),
					ghttp.RespondWithPtr(&statusCode, &body),
				))

		})
		It("Expect 400 status", func() {
			statusConditions, err := ClusterRegistrationStatusConditions(marketplaceClientConfig, marketplaceClientAccount, conditions)
			Expect(err).ShouldNot(HaveOccurred())
			//Expect(updatedMarketplaceConfig.Status.Conditions.GetCondition(marketplacev1alpha1.ConditionRegistrationError)).ShouldNot(BeNil())
			Expect(statusConditions.GetCondition(marketplacev1alpha1.ConditionRegistrationError).Reason).To(Equal(marketplacev1alpha1.ReasonClientError))

		})
	})

	Context("Marketplace Config Status with http status code 300", func() {
		BeforeEach(func() {
			statusCode = 300
			path = "/provisioning/v1/registered-clusters"
			//body = []byte("registered")
			addr = "http://" + server.Addr() + path
			marketplaceClientConfig = &MarketplaceClientConfig{
				Url:   addr,
				Token: "Bearer token",
			}
			marketplaceClientAccount = &MarketplaceClientAccount{
				AccountId:   "accountid",
				ClusterUuid: "clusterid",
			}
			//newMarketplaceConfig = &marketplacev1alpha1.MarketplaceConfig{}
			conditions = &status.Conditions{}
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", path),
					ghttp.RespondWithPtr(&statusCode, &body),
				))

		})
		It("Expect 300 status", func() {
			statusConditions, err := ClusterRegistrationStatusConditions(marketplaceClientConfig, marketplaceClientAccount, conditions)
			Expect(err).ShouldNot(HaveOccurred())
			//Expect(updatedMarketplaceConfig.Status.Conditions.GetCondition(marketplacev1alpha1.ConditionRegistrationError)).ShouldNot(BeNil())
			Expect(statusConditions.GetCondition(marketplacev1alpha1.ConditionRegistrationError).Reason).To(Equal(marketplacev1alpha1.ReasonRegistrationError))

		})
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
	})
})
