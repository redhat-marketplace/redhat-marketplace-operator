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
	"io"
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
	AccountsEndpoint = "account/api/v2/accounts"
)

const (
	RegistrationStatusInstalled = "INSTALLED"
)

type MarketplaceClientConfig struct {
	Url      string
	Token    string
	Insecure bool
	Claims   *utils.MarketplaceClaims
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
	tokenClaims *utils.MarketplaceClaims,
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

const EnvStage = "stage"

// GetJWTTokenClaims will parse JWT token and fetch the rhmAccountId
func GetJWTTokenClaim(jwtToken string) (*utils.MarketplaceClaims, error) {
	// TODO: add verification of public key
	parser := new(jwt.Parser)
	token, parts, err := parser.ParseUnverified(jwtToken, &utils.MarketplaceClaims{})

	if err != nil {
		return nil, err
	}

	// ParseUnverified does not Decode check the signature/parts[2]
	// Decode to check the signature against typos
	if len(parts) != 3 {
		return nil, errors.New("token contains an invalid number of segments")
	}
	_, err = parser.DecodeSegment(parts[2])
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*utils.MarketplaceClaims)

	if !ok {
		return nil, errors.New("token claims is not *MarketplaceClaims")
	}

	return claims, nil
}

type OMAccount struct {
	ID   string `json:"id"`
	Name string `json:"Name"`
}

type UserIdentityInfo struct {
	IAMId string `json:"iamId"`
	Name  string `json:"name"`
}

type AccountsResults struct {
	TotalResults     string           `json:"totalResults"`
	UserIdentityInfo UserIdentityInfo `json:"userIdentityInfo"`
	OMAccounts       []OMAccount      `json:"OMAccounts"`
}

// Fetch basic account info primarily to determine if ibm-entitlement-key has an OMAccount
func (m *MarketplaceClient) RhmAccountExists() (bool, error) {
	u, err := buildQuery(m.endpoint, AccountsEndpoint)
	if err != nil {
		return false, err
	}

	logger.Info("query to check account existence", "query", u.String())

	resp, err := m.httpClient.Get(u.String())
	if err != nil {
		return false, err
	}

	if resp.StatusCode != 200 {
		return false, errors.NewWithDetails("request not successful: "+resp.Status, "statuscode", resp.StatusCode)
	}

	respBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	accountsResults := AccountsResults{}
	err = yaml.Unmarshal(respBodyBytes, &accountsResults)
	if err != nil {
		return false, errors.Wrap(err, "failed to unmarshal accountsResults")
	}

	return (len(accountsResults.OMAccounts) > 0), nil
}
