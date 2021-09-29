package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"

	retryablehttp "github.com/hashicorp/go-retryablehttp"
	rhmotransport "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/transport"

	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	FileServerProductionURL                   = "https://rhm-meterdefinition-file-server.openshift-redhat-marketplace.svc.cluster.local:8200"
	ListForVersionEndpoint                    = "list-for-version"
	GetSystemMeterdefinitionTemplatesEndpoint = "get-system-meterdefs"
	GetMeterdefinitionIndexLabelEndpoint      = "meterdef-index-label"
	GetSystemMeterDefIndexLabelEndpoint       = "system-meterdef-index-label"
	HealthEndpoint                            = "healthz"
)

var (
	CatalogNoContentErr       error = errors.New("not found")
	CatalogPathNotFoundStatus error = errors.New("the path to the file server wasn't found")
	CatalogUnauthorizedErr    error = errors.New("auth error calling file server")
)

type CatalogClient struct {
	sync.Mutex
	Endpoint          *url.URL
	RetryableClient   *retryablehttp.Client
	DeployedNamespace string
	Logger            logr.Logger
	IsAuthSet         bool
}

type CatalogRequest struct {
	CsvName       string `json:"csvName"`
	Version       string `json:"version"`
	CsvNamespace  string `json:"csvNamespace"`
	PackageName   string `json:"packageName"`
	CatalogSource string `json:"catalogSource"`
}

func ProvideCatalogClient(k8sClient client.Client, cfg *config.OperatorConfig, kubeInterface kubernetes.Interface, logr logr.Logger) (*CatalogClient, error) {
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
		RetryableClient:   rc,
		Logger:            logr,
		IsAuthSet:         false,
	}

	return catalogClient, nil
}

func (c *CatalogClient) ListMeterdefintionsFromFileServer(catalogRequest *CatalogRequest) ([]marketplacev1beta1.MeterDefinition, error) {
	c.Logger.Info("retrieving community meterdefinitions", "csvName", catalogRequest.CsvName, "csvVersion", catalogRequest.Version)

	url, err := concatPaths(c.Endpoint.String(), ListForVersionEndpoint)
	if err != nil {
		return nil, err
	}

	requestBody, err := json.Marshal(catalogRequest)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(requestBody)

	response, err := c.RetryableClient.Post(url.String(), "application/json", reader)
	if err != nil {
		c.Logger.Error(err, "Error querying file server for system meter definition")
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		if response.StatusCode == http.StatusNoContent {
			return nil, fmt.Errorf("could not find meterdefinitions for csv: %s and version: %s. response status %s: %w", catalogRequest.CsvName, catalogRequest.Version, response.Status, CatalogNoContentErr)
		}

		return nil, errors.New(fmt.Sprintf("Error querying file server for community meter definition: %s:%d", response.Status, response.StatusCode))

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

func (c *CatalogClient) GetSystemMeterdefs(csv *olmv1alpha1.ClusterServiceVersion) ([]marketplacev1beta1.MeterDefinition, error) {
	c.Logger.Info("retrieving system meterdefinitions", "csvName", csv.Name)

	url, err := concatPaths(c.Endpoint.String(), GetSystemMeterdefinitionTemplatesEndpoint)
	if err != nil {
		return nil, err
	}

	requestBody, err := json.Marshal(csv)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(requestBody)

	response, err := c.RetryableClient.Post(url.String(), "application/json", reader)
	if err != nil {
		c.Logger.Error(err, "Error querying file server for system meter definition")
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		if response.StatusCode == http.StatusNoContent {
			return nil, fmt.Errorf("response status %s: %w", response.Status, CatalogNoContentErr)
		}

		return nil, errors.New(fmt.Sprintf("Error querying file server for system meter definition: %s:%d", response.Status, response.StatusCode))
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

func (c *CatalogClient) GetCommunityMeterdefIndexLabels(csvName string) (map[string]string, error) {
	c.Logger.Info("retrieving community meterdefinition index label")

	url, err := concatPaths(c.Endpoint.String(), GetMeterdefinitionIndexLabelEndpoint, csvName)
	if err != nil {
		return nil, err
	}

	response, err := c.RetryableClient.Get(url.String())
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("Error querying file server for meterdefinition index labels: %s:%d", response.Status, response.StatusCode))
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
		return nil, err
	}

	return labels, nil
}

func (c *CatalogClient) GetSystemMeterDefIndexLabels(csvName string) (map[string]string, error) {
	c.Logger.Info("retrieving system meterdefinition index label")

	url, err := concatPaths(c.Endpoint.String(), GetSystemMeterDefIndexLabelEndpoint, csvName)
	if err != nil {
		return nil, err
	}

	response, err := c.RetryableClient.Get(url.String())
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("Error querying file server for meterdefinition index labels: %s:%d", response.Status, response.StatusCode))
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
		return nil, err
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
	}

	c.RetryableClient.HTTPClient = catalogServerHttpClient

}

func (c *CatalogClient) SetRetryForCatalogClient(k8sclient client.Client, deployedNamespace string, ki kubernetes.Interface, reqLogger logr.Logger) error {
	c.Lock()
	defer c.Unlock()

	if !c.IsAuthSet {
		reqLogger.Info("RetryableClient.HTTPClient is not set, setting")
		httpClient, err := rhmotransport.SetTransportOnHTTPClient(k8sclient, deployedNamespace, ki, reqLogger)
		if err != nil {
			return err
		}

		c.RetryableClient.HTTPClient = httpClient
		c.IsAuthSet = true
	}

	c.RetryableClient.CheckRetry = func(ctx context.Context, resp *http.Response, err error) (bool, error) {
		ok, e := retryablehttp.DefaultRetryPolicy(ctx, resp, err)
		if !ok && resp.StatusCode == http.StatusUnauthorized {
			httpClient, err := rhmotransport.SetTransportOnHTTPClient(k8sclient, deployedNamespace, ki, reqLogger)
			if err != nil {
				return true, err
			}

			c.RetryableClient.HTTPClient = httpClient
			return false, nil
		}

		return ok, e
	}

	return nil
}
