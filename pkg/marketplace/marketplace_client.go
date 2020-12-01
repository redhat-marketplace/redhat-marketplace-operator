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
	"crypto/x509"
	"encoding/json"
	"fmt"
	ioutil "io/ioutil"
	"net/http"
	"net/url"

	"emperror.dev/errors"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/operator-framework/operator-sdk/pkg/status"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

var logger = logf.Log.WithName("marketplace")

const (
	ProductionURL = "https://marketplace.redhat.com"
)

// endpoints
const (
	pullSecretEndpoint   = "provisioning/v1/rhm-operator/rhm-operator-secret"
	registrationEndpoint = "provisioning/v1/registered-clusters"
)

const (
	RegistrationStatusInstalled = "INSTALLED"
)

type MarketplaceClientConfig struct {
	Url      string
	Token    string
	Insecure bool
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
	var tlsConfig *tls.Config

	if clientConfig.Insecure {
		tlsConfig = &tls.Config{InsecureSkipVerify: true}
	} else {
		caCertPool, err := x509.SystemCertPool()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get cert pool")
		}

		tlsConfig = &tls.Config{
			RootCAs: caCertPool,
		}
	}

	var transport http.RoundTripper = &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	if clientConfig.Token != "" {
		transport = WithBearerAuth(transport, clientConfig.Token)
	}

	u, err := url.Parse(clientConfig.Url)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse url")
	}

	return &MarketplaceClient{
		endpoint: u,
		httpClient: http.Client{
			Transport: transport,
		},
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

func buildQuery(u *url.URL, path string, args ...string) (*url.URL, error) {
	buildU := *u
	buildU.Path = path

	if len(args)%2 != 0 {
		return nil, errors.New("query args are not divisible by 2")
	}

	if len(args) > 0 {
		q := buildU.Query()
		for i := 0; i < len(args); i = i + 2 {
			q.Set(args[i], args[i+1])
		}
		buildU.RawQuery = q.Encode()
	}

	return &buildU, nil
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
	Registration       *RegisteredAccount
	RegistrationStatus string
	Err                error
}

func (m *MarketplaceClient) RegistrationStatus(account *MarketplaceClientAccount) (RegistrationStatusOutput, error) {
	if account == nil {
		err := errors.New("account info missing")
		return RegistrationStatusOutput{Err: err}, err
	}

	u, err := buildQuery(m.endpoint, registrationEndpoint,
		"accountId", account.AccountId,
		"uuid", account.ClusterUuid)

	if err != nil {
		return RegistrationStatusOutput{Err: err}, err
	}

	logger.Info("status query", "query", u.String())
	resp, err := m.httpClient.Get(u.String())

	if err != nil || resp.StatusCode != 200 {
		return RegistrationStatusOutput{
			RegistrationStatus: "HttpError",
			Err:                err,
			StatusCode:         resp.StatusCode,
		}, err
	}

	logger.Info("RegistrationStatus status code", "httpstatus", resp.StatusCode)
	clusterDef, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	if err != nil {
		return RegistrationStatusOutput{Err: err}, err
	}

	registrations, err := getRegistrations(string(clusterDef))

	if err != nil {
		return RegistrationStatusOutput{Err: err}, err
	}

	if len(registrations) == 0 {
		return RegistrationStatusOutput{
			StatusCode:         resp.StatusCode,
			RegistrationStatus: "UNREGISTERED",
		}, nil
	}

	for _, registration := range registrations {
		if registration.Uuid == account.ClusterUuid {
			return RegistrationStatusOutput{
				StatusCode:         resp.StatusCode,
				Registration:       &registration,
				RegistrationStatus: registration.Status,
			}, nil
		}
	}

	return RegistrationStatusOutput{
		StatusCode:         resp.StatusCode,
		RegistrationStatus: "UNREGISTERED",
	}, nil
}

func getRegistrations(jsonString string) ([]RegisteredAccount, error) {
	var registeredAccount []RegisteredAccount
	err := json.Unmarshal([]byte(jsonString), &registeredAccount)
	if err != nil {
		logger.Error(err, "Error in GetRegistrationValue Parser for registeredAccount")
		return nil, nil
	}
	return registeredAccount, nil
}

func (resp RegistrationStatusOutput) TransformConfigStatus() status.Conditions {
	conditions := status.NewConditions(status.Condition{
		Type:    marketplacev1alpha1.ConditionRegistered,
		Status:  corev1.ConditionFalse,
		Reason:  marketplacev1alpha1.ReasonRegistrationFailure,
		Message: "Cluster is not registered",
	})

	if resp.StatusCode == 200 {
		if resp.RegistrationStatus == "INSTALLED" {
			message := "Cluster Registered Successfully"
			conditions.SetCondition(status.Condition{
				Type:    marketplacev1alpha1.ConditionRegistered,
				Status:  corev1.ConditionTrue,
				Reason:  marketplacev1alpha1.ReasonRegistrationSuccess,
				Message: message,
			})
		} else {
			message := fmt.Sprintf("Cluster registration pending: %s", resp.RegistrationStatus)
			conditions.SetCondition(status.Condition{
				Type:    marketplacev1alpha1.ConditionRegistered,
				Status:  corev1.ConditionFalse,
				Reason:  marketplacev1alpha1.ReasonRegistrationSuccess,
				Message: message,
			})
		}
	} else {
		msg := http.StatusText(resp.StatusCode)
		msg = fmt.Sprintf("registration failed: %s", msg)
		conditions.SetCondition(status.Condition{
			Type:    marketplacev1alpha1.ConditionRegistered,
			Status:  corev1.ConditionFalse,
			Reason:  marketplacev1alpha1.ReasonRegistrationError,
			Message: msg,
		})
	}

	return conditions
}

func (mhttp *MarketplaceClient) GetMarketplaceSecret() (*corev1.Secret, error) {
	u, err := buildQuery(mhttp.endpoint, pullSecretEndpoint)

	if err != nil {
		return nil, errors.Wrap(err, "failed to build query")
	}

	resp, err := mhttp.httpClient.Get(u.String())
	if err != nil {
		return nil, errors.Wrap(err, "")
	}
	defer resp.Body.Close()

	rhOperatorSecretDef, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "")
	}

	if resp.StatusCode != 200 {
		return nil, errors.NewWithDetails("request not successful", "statuscode", resp.StatusCode)
	}

	newOptSecretObj := corev1.Secret{}
	err = yaml.Unmarshal(rhOperatorSecretDef, &newOptSecretObj)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal secret")
	}

	return &newOptSecretObj, nil
}

type MarketplaceClaims struct {
	AccountID string `json:"rhmAccountId"`
	jwt.StandardClaims
}

// GetAccountIdFromJWTToken will parse JWT token and fetch the rhmAccountId
func GetAccountIdFromJWTToken(jwtToken string) (string, error) {
	// TODO: add verification of public key
	//token, err := jwt.Parse(jwtToken, nil)
	token, _, err := new(jwt.Parser).ParseUnverified(jwtToken, &MarketplaceClaims{})

	if err != nil {
		return "", err
	}

	claims, ok := token.Claims.(*MarketplaceClaims)

	if !ok {
		return "", errors.New("token claims is not *MarketplaceClaims")
	}

	return claims.AccountID, nil
}
