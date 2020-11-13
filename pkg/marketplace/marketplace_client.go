package marketplace

import (
	"encoding/json"
	"errors"
	"fmt"
	ioutil "io/ioutil"
	"net/http"
	"net/url"

	"github.com/operator-framework/operator-sdk/pkg/status"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

var logger = logf.Log.WithName("marketplace")

type MarketplaceClientConfig struct {
	Url   string
	Token string
}

type MarketplaceClientAccount struct {
	AccountId   string
	ClusterUuid string
}

type MarketplaceClient struct {
	endpoint   *url.URL
	httpClient http.Client
}

type RegisteredAccount struct {
	AccountId string
	Uuid      string
	Status    string
}

func NewMarketplaceClient(clientConfig *MarketplaceClientConfig) (*MarketplaceClient, error) {
	var transport http.RoundTripper
	if clientConfig.Token != "" {
		//logger.Info("Token found >>>>>>>>>>>>>>>>", "clienConfigToken", clientConfig.Token)
		transport = WithBearerAuth(transport, clientConfig.Token)
	}
	u, err := url.Parse(clientConfig.Url)
	if err != nil {
		return nil, err
	}
	//logger.Info("MarketplaceClient creating and returning >>>>>>>>>>>>>>>")
	return &MarketplaceClient{
		endpoint:   u,
		httpClient: http.Client{Transport: transport},
	}, nil
}

type withHeader struct {
	http.Header
	rt http.RoundTripper
}

func WithBearerAuth(rt http.RoundTripper, token string) http.RoundTripper {
	addHead := WithHeader(rt)
	addHead.Header.Set("Authorization", "Bearer "+token)
	return addHead
}

func WithQuery(u *url.URL, clientAccount *MarketplaceClientAccount) *url.URL {
	q := u.Query()
	//logger.Info("MarketplaceClientAccount query >>>>>> : ", "accountId", clientAccount.AccountId, "uuid", clientAccount.ClusterUuid)
	q.Set("accountId", clientAccount.AccountId)
	q.Set("uuid", clientAccount.ClusterUuid)
	u.RawQuery = q.Encode()
	return u
}

func WithHeader(rt http.RoundTripper) withHeader {
	if rt == nil {
		rt = http.DefaultTransport
	}

	return withHeader{Header: make(http.Header), rt: rt}
}

func (h withHeader) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range h.Header {
		req.Header[k] = v
	}

	return h.rt.RoundTrip(req)
}

type RegistrationStatusInput struct {
	MarketplaceClientAccount
}

type RegistrationStatusOutput struct {
	StatusCode         int
	RegistrationStatus string
	//anything else you need
}

func (m *MarketplaceClient) RegistrationStatus(account *MarketplaceClientAccount) RegistrationStatusOutput {
	//	logger.Info("RegistrationStatus method called >>>>>>> ")
	u := WithQuery(m.endpoint, account)
	//logger.Info("RegistrationStatus method calling query >>>>>>> ", "url", u.String())
	resp, err := m.httpClient.Get(u.String())
	logger.Info("RegistrationStatus status code : ", "httpstatus", resp.StatusCode)
	if err != nil {
		return RegistrationStatusOutput{
			StatusCode:         resp.StatusCode,
			RegistrationStatus: "Request not Complete, Http error",
		}
	}
	defer resp.Body.Close()

	clusterDef, err := ioutil.ReadAll(resp.Body)
	//logger.Info("clusterDef Json value for registeredAccount: ", "clusterDef", clusterDef)

	if err != nil {
		return RegistrationStatusOutput{
			StatusCode:         resp.StatusCode,
			RegistrationStatus: "Error with JSON response",
		}
	}

	if string(clusterDef) == "[]" {
		logger.Info("Account is not registered")
		return RegistrationStatusOutput{
			StatusCode:         resp.StatusCode,
			RegistrationStatus: "NotRegistered",
		}
	}
	registrationStatus := GetRegistrationValue(string(clusterDef))
	return RegistrationStatusOutput{
		StatusCode:         resp.StatusCode,
		RegistrationStatus: registrationStatus,
	}
}

func GetRegistrationValue(jsonString string) string {
	var registeredAccount []RegisteredAccount
	err := json.Unmarshal([]byte(jsonString), &registeredAccount)
	if err != nil {
		logger.Error(err, "Error in GetRegistrationValue Parser for registeredAccount")
	}
	//logger.Info("GetRegistrationValue Json value for registeredAccount: ", "registeredAccount", registeredAccount)
	account := registeredAccount[0]
	return account.Status
}
func ClusterRegistrationStatus(marketplaceClientConfig *MarketplaceClientConfig, marketplaceClientAccount *MarketplaceClientAccount) (*RegistrationStatusOutput, error) {
	newMarketPlaceClient, err := NewMarketplaceClient(marketplaceClientConfig)
	//registrationStatusOutput = &RegistrationStatusOutput{}
	if err != nil {
		return nil, err
	}
	registrationStatusOutput := newMarketPlaceClient.RegistrationStatus(marketplaceClientAccount)
	return &registrationStatusOutput, nil
}
func ClusterRegistrationStatusConditions(marketplaceClientConfig *MarketplaceClientConfig, marketplaceClientAccount *MarketplaceClientAccount, conditions *status.Conditions) (*status.Conditions, error) {
	conditions.RemoveCondition(marketplacev1alpha1.ConditionRegistered)
	conditions.RemoveCondition(marketplacev1alpha1.ConditionRegistrationError)
	newMarketPlaceClient, err := NewMarketplaceClient(marketplaceClientConfig)
	if err != nil {
		message := "Http Client Error with Cluster Registration "
		conditions.SetCondition(status.Condition{
			Type:    marketplacev1alpha1.ConditionRegistrationError,
			Status:  corev1.ConditionTrue,
			Reason:  marketplacev1alpha1.ReasonRegistrationError,
			Message: message,
		})
		return conditions, err
	}
	registrationStatusOutput := newMarketPlaceClient.RegistrationStatus(marketplaceClientAccount)
	condition := TransformConfigStatus(registrationStatusOutput)
	conditions.SetCondition(condition)
	return conditions, nil
}

func TransformConfigStatus(resp RegistrationStatusOutput) status.Condition {
	if resp.StatusCode == 200 && resp.RegistrationStatus == "INSTALLED" {
		//reqLogger.Info("Cluster Registered")
		message := "Cluster Registered Successfully"
		return status.Condition{
			Type:    marketplacev1alpha1.ConditionRegistered,
			Status:  corev1.ConditionFalse,
			Reason:  marketplacev1alpha1.ReasonRegistrationStatus,
			Message: message,
		}
	} else if resp.StatusCode == 500 {
		message := "Registration Service Unavailable"
		return status.Condition{
			Type:    marketplacev1alpha1.ConditionRegistrationError,
			Status:  corev1.ConditionTrue,
			Reason:  marketplacev1alpha1.ReasonServiceUnavailable,
			Message: message,
		}
	} else if resp.StatusCode == 408 {
		message := "DNS/Internet gateway failure - Disconnected env "
		return status.Condition{
			Type:    marketplacev1alpha1.ConditionRegistrationError,
			Status:  corev1.ConditionTrue,
			Reason:  marketplacev1alpha1.ReasonInternetDisconnected,
			Message: message,
		}
	} else if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		message := "Client Error, Please check connection to registration service "
		return status.Condition{
			Type:    marketplacev1alpha1.ConditionRegistrationError,
			Status:  corev1.ConditionTrue,
			Reason:  marketplacev1alpha1.ReasonClientError,
			Message: message,
		}
	} else {
		message := fmt.Sprint("Error with Cluster Registration with http status code: ", resp.StatusCode)
		return status.Condition{
			Type:    marketplacev1alpha1.ConditionRegistrationError,
			Status:  corev1.ConditionTrue,
			Reason:  marketplacev1alpha1.ReasonRegistrationError,
			Message: message,
		}
	}

}

func GetMarketPlaceSecret(marketplaceClientConfig *MarketplaceClientConfig) ([]byte, error) {
	newMarketPlaceClient, err := NewMarketplaceClient(marketplaceClientConfig)
	u := newMarketPlaceClient.endpoint
	resp, err := newMarketPlaceClient.httpClient.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	rhOperatorSecretDef, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return rhOperatorSecretDef, err
	}
	if resp.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("%s%d", "Request not successful, http status code: ", resp.StatusCode))
	}

	data, err := yaml.YAMLToJSON(rhOperatorSecretDef) //Converting Yaml to JSON so it can parse to secret object
	if err != nil {
		return data, err
	}

	return data, nil

}
