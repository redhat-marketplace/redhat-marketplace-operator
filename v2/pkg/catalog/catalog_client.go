// Copyright 2021 IBM Corp.
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

package catalog

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/goph/emperror"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/pkg/errors"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"

	retryablehttp "github.com/hashicorp/go-retryablehttp"
	rhmotransport "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/rhmotransport"

	"k8s.io/apimachinery/pkg/util/yaml"
)

const (
	FileServerProductionURL                             = "https://rhm-meterdefinition-file-server.openshift-redhat-marketplace.svc.cluster.local:8200"
	GetCommunityMeterdefinitionsEndpoint                = "get-community-meterdefs"
	GetSystemMeterdefinitionTemplatesEndpoint           = "get-system-meterdefs"
	GetCommunityMeterdefinitionIndexLabelEndpoint       = "community-meterdef-index"
	GetSystemMeterDefIndexLabelEndpoint                 = "system-meterdef-index"
	GetGlobalCommunityMeterdefinitionIndexLabelEndpoint = "global-community-meterdef-index"
	GetGlobalSystemMeterDefIndexLabelEndpoint           = "global-system-meterdef-index"
	HealthEndpoint                                      = "healthz"
)

var (
	// used in the deploymentconfig controller to determine if an isv has deleted their meterdefs
	ErrCatalogNoContent error = errors.New("not found")

	// TODO: leaving this in here for now in case we need it
	ErrCatalogUnauthorized error = errors.New("auth error on call to meterdefinition catalog server")
)

type CatalogClient struct {
	sync.Mutex
	Endpoint *url.URL
	*retryablehttp.Client
	authBuilder       rhmotransport.IAuthBuilder
	DeployedNamespace string
	IsTransportSet    bool
	UseSecureClient   bool
	Logger            logr.Logger
}

/*
CatalogRequest defined in the file server repo
*/
type CatalogRequest struct {
	SubInfo `json:"subInfo"`
	CSVInfo `json:"csvInfo"`
}

type SubInfo struct {
	PackageName   string `json:"packageName"`
	CatalogSource string `json:"catalogSource"`
}

type CSVInfo struct {
	Name      string `json:"csvName"`
	Namespace string `json:"csvNamespace"`
	Version   string `json:"version"`
}

func ProvideCatalogClient(authBuilderInterface rhmotransport.IAuthBuilder, cfg *config.OperatorConfig, logr logr.Logger) (*CatalogClient, error) {
	fileServerURL := FileServerProductionURL

	if cfg.FileServerURL != "" {
		fileServerURL = cfg.FileServerURL
	}

	url, err := url.Parse(fileServerURL)
	if err != nil {
		return nil, err
	}

	rc := retryablehttp.NewClient()
	rc.RetryWaitMax = 5000 * time.Millisecond
	rc.RetryWaitMin = 3000 * time.Millisecond
	rc.RetryMax = 5

	catalogClient := &CatalogClient{
		Endpoint:          url,
		DeployedNamespace: cfg.DeployedNamespace,
		Client:            rc,
		authBuilder:       authBuilderInterface,
		Logger:            logr,
		UseSecureClient:   true,
		IsTransportSet:    false,
	}

	return catalogClient, nil
}

func (c *CatalogClient) ListMeterdefintionsFromFileServer(catalogRequest *CatalogRequest) ([]marketplacev1beta1.MeterDefinition, error) {
	c.Logger.Info("retrieving community meterdefinitions", "csvName", catalogRequest.Name, "csvVersion", catalogRequest.Version)

	err := c.init()
	if err != nil {
		return nil, err
	}

	url, err := concatPaths(c.Endpoint.String(), GetCommunityMeterdefinitionsEndpoint)
	if err != nil {
		return nil, err
	}

	requestBody, err := json.Marshal(catalogRequest)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(requestBody)

	response, err := c.Post(url.String(), "application/json", reader)
	if err != nil {
		c.Logger.Error(err, "Error querying file server for system meter definition")
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		if response.StatusCode == http.StatusNoContent {
			return nil, fmt.Errorf("could not find meterdefinitions for csv: %s and version: %s. response status %s: %w", catalogRequest.Name, catalogRequest.Version, response.Status, ErrCatalogNoContent)
		}

		return nil, fmt.Errorf("Error querying file server for community meter definitions: %s", response.Status)
	}

	c.Logger.Info("response", "response", response)

	mdefSlice := []marketplacev1beta1.MeterDefinition{}

	defer response.Body.Close()

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		c.Logger.Error(err, "error reading body")
		return nil, err
	}

	err = yaml.NewYAMLOrJSONDecoder(bytes.NewReader(responseData), 100).Decode(&mdefSlice)
	if err != nil {
		c.Logger.Error(err, "error decoding response from ListMeterdefinitions()")
		return nil, err
	}

	return mdefSlice, nil
}

func (c *CatalogClient) GetSystemMeterdefs(csv *olmv1alpha1.ClusterServiceVersion, packageName string, catalogSource string) ([]marketplacev1beta1.MeterDefinition, error) {
	c.Logger.Info("retrieving system meterdefinitions for csv", "csv", csv.Name)

	err := c.init()
	if err != nil {
		return nil, err
	}

	url, err := concatPaths(c.Endpoint.String(), GetSystemMeterdefinitionTemplatesEndpoint, packageName, catalogSource)
	if err != nil {
		return nil, err
	}

	requestBody, err := json.Marshal(csv)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(requestBody)

	response, err := c.Post(url.String(), "application/json", reader)
	if err != nil {
		c.Logger.Error(err, "Error querying file server for system meter definition")
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		if response.StatusCode == http.StatusNoContent {
			return nil, fmt.Errorf("response status %s: %w", response.Status, ErrCatalogNoContent)
		}

		return nil, fmt.Errorf("Error querying file server for system meter definition: %s", response.Status)
	}

	defer response.Body.Close()

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		c.Logger.Error(err, "error reading body")
		return nil, err
	}

	mdefSlice := []marketplacev1beta1.MeterDefinition{}

	err = yaml.NewYAMLOrJSONDecoder(bytes.NewReader(responseData), 100).Decode(&mdefSlice)
	if err != nil {
		c.Logger.Error(err, "error decoding response from GetSystemMeterdefinitions()")
		return nil, err
	}

	return mdefSlice, nil
}

func (c *CatalogClient) GetCommunityMeterdefIndexLabels(csvName string, packageName string, catalogSourceName string) (map[string]string, error) {
	c.Logger.Info("retrieving community meterdefinition index label")

	err := c.init()
	if err != nil {
		return nil, err
	}

	url, err := concatPaths(c.Endpoint.String(), GetCommunityMeterdefinitionIndexLabelEndpoint, csvName, packageName, catalogSourceName)
	if err != nil {
		return nil, err
	}

	response, err := c.Get(url.String())
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Error querying file server for meterdefinition index labels: %s", response.Status)
	}

	defer response.Body.Close()

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		c.Logger.Error(err, "error reading body")
		return nil, err
	}

	labels := map[string]string{}
	err = json.Unmarshal(data, &labels)
	if err != nil {
		return nil, emperror.Wrap(err, "unmarshaling error")
	}

	return labels, nil
}

func (c *CatalogClient) GetSystemMeterDefIndexLabels(csvName string, packageName string, catalogSourceName string) (map[string]string, error) {
	c.Logger.Info("retrieving system meterdefinition index label")

	err := c.init()
	if err != nil {
		return nil, err
	}

	url, err := concatPaths(c.Endpoint.String(), GetSystemMeterDefIndexLabelEndpoint, csvName, packageName, catalogSourceName)
	if err != nil {
		return nil, err
	}

	response, err := c.Get(url.String())
	if err != nil {
		return nil, err
	}

	c.Logger.Info("successful response")

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Error querying file server for meterdefinition index labels: %s", response.Status)
	}

	defer response.Body.Close()

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		c.Logger.Error(err, "error reading body")
		return nil, err
	}

	labels := map[string]string{}
	err = json.Unmarshal(data, &labels)
	if err != nil {
		return nil, emperror.Wrap(err, "unmarshaling error")
	}

	return labels, nil
}

func (c *CatalogClient) GetGlobalCommunityMeterdefIndexLabel() (map[string]string, error) {
	c.Logger.Info("retrieving community meterdefinition index label")

	err := c.init()
	if err != nil {
		return nil, err
	}

	url, err := concatPaths(c.Endpoint.String(), GetGlobalCommunityMeterdefinitionIndexLabelEndpoint)
	if err != nil {
		return nil, err
	}

	response, err := c.Get(url.String())
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Error querying file server for global community meterdefinition index labels: %s", response.Status)
	}

	defer response.Body.Close()

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		c.Logger.Error(err, "error reading body")
		return nil, err
	}

	labels := map[string]string{}
	err = json.Unmarshal(data, &labels)
	if err != nil {
		return nil, emperror.Wrap(err, "unmarshaling error")
	}

	return labels, nil
}

func (c *CatalogClient) GetGlobalSystemMeterDefIndexLabels() (map[string]string, error) {
	c.Logger.Info("retrieving global system meterdefinition index label")

	err := c.init()
	if err != nil {
		return nil, err
	}

	url, err := concatPaths(c.Endpoint.String(), GetGlobalSystemMeterDefIndexLabelEndpoint)
	if err != nil {
		return nil, err
	}

	response, err := c.Get(url.String())
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Error querying file server for global system meterdefinition index labels: %s", response.Status)
	}

	defer response.Body.Close()

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		c.Logger.Error(err, "error reading body")
		return nil, err
	}

	labels := map[string]string{}
	err = json.Unmarshal(data, &labels)
	if err != nil {
		return nil, emperror.Wrap(err, "unmarshaling error")
	}

	return labels, nil
}

func concatPaths(basePath string, paths ...string) (*url.URL, error) {

	u, err := url.Parse(basePath)

	if err != nil {
		return nil, err
	}

	temp := append([]string{u.Path}, paths...)

	concatenatedPaths := path.Join(temp...)

	u.Path = concatenatedPaths

	return u, nil
}

func (c *CatalogClient) UseInsecureClient() {

	catalogServerHttpClient := &http.Client{
		Timeout: 1 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				CipherSuites: []uint16{tls.TLS_AES_128_GCM_SHA256,
					tls.TLS_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384},
				MinVersion: tls.VersionTLS12,
			},
		},
	}

	c.HTTPClient = catalogServerHttpClient
	c.UseSecureClient = false
}

// initializes the httpclient on the retryablehttp and defines what conditions we need to retry on
// default retry will retry on connection errors, or if a 500-range response code is received (except 501)
// add authbuilder to CatalogClient, make this an init function that gets called on client call functions
func (c *CatalogClient) init() error {
	c.Lock()
	defer c.Unlock()

	if c.UseSecureClient {
		// TODO: was having trouble hitting a nil condition to determine if transport has been set by us or not, using a flag for now
		if !c.IsTransportSet {
			c.Logger.Info("RetryableClient.HTTPClient is not set, setting")

			httpClient, err := rhmotransport.SetTransportForKubeServiceAuth(c.authBuilder, c.Logger)
			if err != nil {
				c.Logger.Error(err, "error setting transport")
				return err
			}

			c.HTTPClient = httpClient
			c.IsTransportSet = true
		}

		c.CheckRetry = func(ctx context.Context, resp *http.Response, err error) (bool, error) {
			ok, err := retryablehttp.ErrorPropagatedRetryPolicy(ctx, resp, err)
			if !ok && resp.StatusCode == http.StatusUnauthorized {
				httpClient, err := rhmotransport.SetTransportForKubeServiceAuth(c.authBuilder, c.Logger)
				if err != nil {
					return true, err
				}

				c.HTTPClient = httpClient
				// retry after setting auth
				// set return err if all retry attempts fail
				err = fmt.Errorf("%w. Call returned with: %s", ErrCatalogUnauthorized, resp.Status)
				return true, err
			}

			return ok, err
		}
	} else if !c.UseSecureClient {
		c.Logger.Info("using insecure client skipping auth configuration")
	}

	return nil
}
