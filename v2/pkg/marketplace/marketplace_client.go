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
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	ioutil "io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"emperror.dev/errors"
	jwt "github.com/golang-jwt/jwt/v5"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	status "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	corev1 "k8s.io/api/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

var logger = logf.Log.WithName("marketplace")

// endpoints
const (
	PullSecretEndpoint       = "provisioning/v1/rhm-operator/rhm-operator-secret"
	RegistrationEndpoint     = "provisioning/v1/registered-clusters"
	MigrateChildRRS3Endpoint = "provisioning/v1/child-yaml-migration"
	AuthenticationEndpoint   = "subscriptions/api/v1/keys/authentication"
)

const (
	RegistrationStatusInstalled = "INSTALLED"
)

type MarketplaceClientConfig struct {
	Url      string
	Token    string
	Insecure bool
	Claims   *MarketplaceClaims
}

type MarketplaceClientAccount struct {
	AccountId   string `json:"accountId"`
	ClusterUuid string `json:"uuid"`
}

type MarketplaceClient struct {
	endpoint   *url.URL
	httpClient http.Client
}

type RegisteredAccount struct {
	Id        string `json:"_id"`
	AccountId string
	Uuid      string
	Status    string
}

type MarketplaceClientBuilder struct {
	Url      string
	Insecure bool
}

type EntitlementKey struct {
	Auths map[string]Auth `json:"auths"`
}

type Auth struct {
	UserName string `json:"username"`
	Password string `json:"password"`
	Auth     string `json:"auth"`
}

func NewMarketplaceClientBuilder(cfg *config.OperatorConfig) *MarketplaceClientBuilder {
	builder := &MarketplaceClientBuilder{}

	builder.Url = utils.ProductionURL

	if cfg.URL != "" {
		builder.Url = cfg.URL
		logger.V(2).Info("using env override for marketplace url", "url", builder.Url)
	}

	logger.V(2).Info("marketplace url set to", "url", builder.Url)
	builder.Insecure = cfg.InsecureClient

	return builder
}

func (b *MarketplaceClientBuilder) NewMarketplaceClient(
	token string,
	tokenClaims *MarketplaceClaims,
) (*MarketplaceClient, error) {
	var tlsConfig *tls.Config
	marketplaceURL := b.Url

	if tokenClaims != nil &&
		b.Url == utils.ProductionURL &&
		strings.ToLower(tokenClaims.Env) == strings.ToLower(EnvStage) {
		marketplaceURL = utils.StageURL
		logger.V(2).Info("using stage for marketplace url", "url", marketplaceURL)
	}

	caCertPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cert pool")
	}

	tlsConfig = &tls.Config{
		RootCAs:            caCertPool,
		InsecureSkipVerify: b.Insecure,
		CipherSuites: []uint16{tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384},
		MinVersion: tls.VersionTLS12,
	}

	var transport http.RoundTripper = &http.Transport{
		TLSClientConfig: tlsConfig,
		Proxy:           http.ProxyFromEnvironment,
	}

	if token == "" {
		return nil, errors.New("transport is empty for production")
	}

	transport = WithBearerAuth(transport, token)

	u, err := url.Parse(marketplaceURL)
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

	// Don't check Registration if there is no rhmAccountID
	// Typical case for IEK with no RHM account
	if len(account.AccountId) == 0 {
		return RegistrationStatusOutput{
			RegistrationStatus: "NoRHMAccountID",
		}, nil
	}

	if len(account.ClusterUuid) == 0 {
		return RegistrationStatusOutput{
			RegistrationStatus: "NoClusterUuid",
		}, nil
	}

	u, err := buildQuery(m.endpoint, RegistrationEndpoint,
		"accountId", account.AccountId,
		"uuid", account.ClusterUuid)

	if err != nil {
		return RegistrationStatusOutput{Err: err}, err
	}

	logger.Info("status query", "query", u.String())
	resp, err := m.httpClient.Get(u.String())

	if err != nil {
		return RegistrationStatusOutput{
			RegistrationStatus: "HttpError",
			Err:                err,
			StatusCode:         http.StatusInternalServerError,
		}, err
	}
	if resp.StatusCode != 200 {
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
	} else if resp.StatusCode != 0 {
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
	u, err := buildQuery(mhttp.endpoint, PullSecretEndpoint)

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
		return nil, errors.NewWithDetails("request not successful: "+resp.Status, "statuscode", resp.StatusCode)
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
	Password  string `json:"password,omitempty"`
	APIKey    string `json:"iam_apikey,omitempty"`
	Env       string `json:"env,omitempty"`
	jwt.RegisteredClaims
}

const EnvStage = "stage"

// GetJWTTokenClaims will parse JWT token and fetch the rhmAccountId
func GetJWTTokenClaim(jwtToken string) (*MarketplaceClaims, error) {
	// TODO: add verification of public key
	token, _, err := new(jwt.Parser).ParseUnverified(jwtToken, &MarketplaceClaims{})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*MarketplaceClaims)

	if !ok {
		return nil, errors.New("token claims is not *MarketplaceClaims")
	}

	return claims, nil
}

func (m *MarketplaceClient) getClusterObjID(account *MarketplaceClientAccount) (string, error) {
	u, err := buildQuery(m.endpoint, RegistrationEndpoint,
		"accountId", account.AccountId,
		"uuid", account.ClusterUuid)

	if err != nil {
		return "", err
	}

	logger.Info("get cluster objId query", "query", u.String())
	resp, err := m.httpClient.Get(u.String())
	if err != nil {
		return "", err
	}

	clusterDef, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	if err != nil {
		return "", err
	}

	registrations, err := getRegistrations(string(clusterDef))
	if err != nil {
		return "", err
	}

	var objId string
	for _, registration := range registrations {
		if registration.Uuid == account.ClusterUuid {
			objId = registration.Id
		}
	}
	return objId, nil
}

func (mhttp *MarketplaceClient) MigrateChildRRS3(account *MarketplaceClientAccount) error {
	u, err := buildQuery(mhttp.endpoint, MigrateChildRRS3Endpoint)

	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}

	requestBody, err := json.Marshal(account)
	if err != nil {
		return errors.Wrap(err, "")
	}

	resp, err := mhttp.httpClient.Post(u.String(), "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return errors.Wrap(err, "")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.NewWithDetails("request not successful: "+resp.Status, "statuscode", resp.StatusCode)
	}

	return nil
}

func (m *MarketplaceClient) UnRegister(account *MarketplaceClientAccount) (RegistrationStatusOutput, error) {
	if account == nil {
		err := errors.New("account info missing")
		return RegistrationStatusOutput{Err: err}, err
	}

	objID, err := m.getClusterObjID(account)
	if err != nil {
		return RegistrationStatusOutput{Err: err}, err
	}

	if err != nil {
		return RegistrationStatusOutput{Err: err}, err
	}

	url := m.endpoint.String() + "/" + RegistrationEndpoint + "/" + objID

	logger.Info("status query", "query", url)

	requestBody, err := json.Marshal(map[string]string{
		"accountId": account.AccountId,
		"status":    "TO_BE_UNREGISTERED",
	})

	if err != nil {
		return RegistrationStatusOutput{Err: err}, err
	}

	patchReq, err := http.NewRequest("PATCH", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return RegistrationStatusOutput{Err: err}, err
	}

	patchReq.Header.Set("Content-Type", "application/json")
	resp, err := m.httpClient.Do(patchReq)
	if err != nil {
		return RegistrationStatusOutput{
			RegistrationStatus: "HttpError",
			Err:                err,
			StatusCode:         http.StatusInternalServerError,
		}, err
	} else if resp.StatusCode == 200 {
		return RegistrationStatusOutput{
			StatusCode:         resp.StatusCode,
			RegistrationStatus: "UNREGISTERED",
		}, nil
	} else {
		return RegistrationStatusOutput{
			RegistrationStatus: "HttpError",
			Err:                err,
			StatusCode:         resp.StatusCode,
		}, err
	}
}

func (m *MarketplaceClient) RhmAccountExists() (bool, error) {
	u, err := buildQuery(m.endpoint, AuthenticationEndpoint)
	if err != nil {
		return false, err
	}

	logger.Info("query to check rhmAccount existence", "query", u.String())

	requestBody, err := json.Marshal(map[string]bool{
		"createAccount":              false,
		"sendEmailOnAccountCreation": false,
	})
	if err != nil {
		return false, errors.New("intenalError: json.Marshal")
	}

	requestBodyBuffer := bytes.NewBuffer(requestBody)
	if err != nil {
		return false, errors.New("intenalError: NewBuffer")
	}

	resp, err := m.httpClient.Post(u.String(), "application/json", requestBodyBuffer)
	if err != nil {
		return false, err
	}

	if resp.StatusCode == 200 {
		return true, nil
	}
	if resp.StatusCode == 206 {
		return false, nil
	}

	return false, errors.New("unexpected response status " + resp.Status)
}
