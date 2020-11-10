package marketplaceconfig

import (
	"fmt"
	ioutil "io/ioutil"
	"net/http"
	"net/url"

	"github.com/operator-framework/operator-sdk/pkg/status"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

type MarketplaceClientConfig struct {
	Url         string
	Token       string
	AccountId   string
	ClusterUuid string
}

func ClusterRegistrationStatus(marketplaceClientConfig *MarketplaceClientConfig, marketplaceConfig marketplacev1alpha1.MarketplaceConfig) (marketplacev1alpha1.MarketplaceConfig, error) {
	httpClient := &http.Client{}
	accountId := marketplaceClientConfig.AccountId
	clusterUuid := marketplaceClientConfig.ClusterUuid
	u, err := url.Parse(marketplaceClientConfig.Url)
	if err != nil {
		return marketplaceConfig, err
	}
	q := u.Query()
	q.Set("accountId", accountId)
	q.Set("uuid", clusterUuid)
	u.RawQuery = q.Encode()
	req, _ := http.NewRequest("GET", u.String(), nil)
	req.Header.Add("Authorization", marketplaceClientConfig.Token)
	resp, err := httpClient.Do(req)
	if err != nil {
		return marketplaceConfig, err
	}
	defer resp.Body.Close()

	marketplaceConfig, err = FindMarketPlaceConfigStatus(*resp, marketplaceConfig)

	return marketplaceConfig, nil
}

func FindMarketPlaceConfigStatus(resp http.Response, marketplaceConfig marketplacev1alpha1.MarketplaceConfig) (marketplacev1alpha1.MarketplaceConfig, error) {
	clusterDef, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return marketplaceConfig, err
	}
	if resp.StatusCode == 200 && string(clusterDef) != "" {
		//reqLogger.Info("Cluster Registered")
		message := "Cluster Registered Successfully"
		marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
			Type:    marketplacev1alpha1.ConditionRegistered,
			Status:  corev1.ConditionFalse,
			Reason:  marketplacev1alpha1.ReasonRegistrationStatus,
			Message: message,
		})
	} else if resp.StatusCode == 500 {
		message := "Registration Service Unavailable"
		marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
			Type:    marketplacev1alpha1.ConditionError,
			Status:  corev1.ConditionTrue,
			Reason:  marketplacev1alpha1.ReasonServiceUnavailable,
			Message: message,
		})
	} else if resp.StatusCode == 408 {
		message := "DNS/Internet gateway failure - Disconnected env "
		marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
			Type:    marketplacev1alpha1.ConditionError,
			Status:  corev1.ConditionTrue,
			Reason:  marketplacev1alpha1.ReasonInternetDisconnected,
			Message: message,
		})
	} else if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		message := "Client Error, Please check connection to registration service "
		marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
			Type:    marketplacev1alpha1.ConditionError,
			Status:  corev1.ConditionTrue,
			Reason:  marketplacev1alpha1.ReasonClientError,
			Message: message,
		})
	} else {
		message := fmt.Sprint("Error with Cluster Registration with http status code: ", resp.StatusCode)
		marketplaceConfig.Status.Conditions.SetCondition(status.Condition{
			Type:    marketplacev1alpha1.ConditionError,
			Status:  corev1.ConditionTrue,
			Reason:  marketplacev1alpha1.ReasonRegistrationError,
			Message: message,
		})
	}
	return marketplaceConfig, nil
}
