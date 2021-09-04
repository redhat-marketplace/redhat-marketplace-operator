package catalog

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"

	emperror "emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	prom "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/prometheus"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"

	// . "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	FileServerProductionURL                   = "https://rhm-meterdefinition-file-server.openshift-redhat-marketplace.svc.cluster.local:8200"
	ListForVersionEndpoint                    = "list-for-version"
	GetSystemMeterdefinitionTemplatesEndpoint = "get-system-meterdefs"
	GetMeterdefinitionIndexLabelEndpoint      = "meterdef-index-label"
	GetSystemMeterDefIndexLabelEndpoint 	  = "system-meterdef-index-label"
	HealthEndpoint							  = "healthz" 
)

var (
	CatalogNoContentErr       error = errors.New("not found")
	CatalogPathNotFoundStatus error = errors.New("the path to the file server wasn't found")
	CatalogUnauthorizedErr    error = errors.New("auth error calling file server")
)

type CatalogClient struct {
	sync.Mutex
	Endpoint          *url.URL
	HttpClient        *http.Client
	K8sClient         client.Client
	KubeInterface     kubernetes.Interface
	DeployedNamespace string
}

func ProvideCatalogClient(k8sClient client.Client, cfg *config.OperatorConfig, kubeInterface kubernetes.Interface) (*CatalogClient, error) {
	fileServerUrl := FileServerProductionURL

	if cfg.FileServerURL != "" {
		fileServerUrl = cfg.FileServerURL
	}

	url, err := url.Parse(fileServerUrl)
	if err != nil {
		return nil, err
	}

	return &CatalogClient{
		Endpoint:          url,
		DeployedNamespace: cfg.DeployedNamespace,
		KubeInterface:     kubeInterface,
		K8sClient:         k8sClient,
	}, nil
}

func (c *CatalogClient) UseInsecureClient() {

	catalogServerHttpClient := &http.Client{
		Timeout: 1 * time.Second,
	}

	c.HttpClient = catalogServerHttpClient

}

func (c *CatalogClient) SetTransport(reqLogger logr.Logger) error {
	c.Lock()
	defer c.Unlock()

	service, err := c.getCatalogServerService(reqLogger)
	if err != nil {
		return err
	}

	cert, err := c.getCertFromConfigMap(reqLogger)
	if err != nil {
		return err
	}

	saClient := prom.NewServiceAccountClient(c.DeployedNamespace, c.KubeInterface)
	authToken, err := saClient.NewServiceAccountToken(utils.OPERATOR_SERVICE_ACCOUNT, utils.FileServerAudience, 3600, reqLogger)
	if err != nil {
		return err
	}

	if service != nil && len(cert) != 0 && authToken != "" {
		caCertPool, err := x509.SystemCertPool()
		if err != nil {
			return err
		}

		ok := caCertPool.AppendCertsFromPEM(cert)
		if !ok {
			err = emperror.New("failed to append cert to cert pool")
			reqLogger.Error(err, "cert pool error")
			return err
		}

		tlsConfig := &tls.Config{
			RootCAs: caCertPool,
		}

		var transport http.RoundTripper = &http.Transport{
			TLSClientConfig: tlsConfig,
			Proxy:           http.ProxyFromEnvironment,
		}

		transport = WithBearerAuth(transport, authToken)

		catalogServerHttpClient := &http.Client{
			Transport: transport,
			Timeout:   1 * time.Second,
		}

		c.HttpClient = catalogServerHttpClient
	}

	return nil
}

func (c *CatalogClient) CheckAuth (reqLogger logr.Logger) error {
	reqLogger.Info("checking file server auth")

	if c.HttpClient == nil {
		reqLogger.Info("setting transport on catalog client")

		err := c.SetTransport(reqLogger)
		if err != nil {
			err = errors.Wrap(err,"error setting transport on Catalog Client")
			return err
		}
	}
	 
	err := c.Ping(reqLogger)
	if err != nil {
		if errors.Is(err, CatalogUnauthorizedErr) {
			err := c.SetTransport(reqLogger)
			if err != nil {
				err = errors.Wrap(err,"error setting transport on Catalog Client")
				return err
			}
		}

		return err
	}
	

	return nil
}

func (c *CatalogClient) Ping(reqLogger logr.Logger) error {
	reqLogger.Info("pinging file server")

	url, err := concatPaths(c.Endpoint.String(), HealthEndpoint)
	if err != nil {
		return err
	}

	reqLogger.Info("making call to", "url", url.String())

	response, err := c.HttpClient.Get(url.String())
	if err != nil {
		return err
	}

	if response.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("response status %s: %w", response.Status, CatalogUnauthorizedErr)
	}

	return nil
}

func (c *CatalogClient) ListMeterdefintionsFromFileServer(csvName string, version string, namespace string, reqLogger logr.Logger) ([]marketplacev1beta1.MeterDefinition, error) {
	reqLogger.Info("retrieving meterdefinitions", "csvName", csvName, "csvVersion", version)

	url, err := concatPaths(c.Endpoint.String(), ListForVersionEndpoint, csvName, version, namespace)
	if err != nil {
		return nil, err
	}

	reqLogger.Info("making call to", "url", url.String())

	response, err := c.HttpClient.Get(url.String())
	if err != nil {
		reqLogger.Error(err, "Error on GET to Catalog Server")
		return nil, err
	}

	if response.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("response status %s: %w", response.Status, CatalogUnauthorizedErr)
	}

	if response.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("response status %s: %w", response.Status, CatalogPathNotFoundStatus)
	}

	if response.StatusCode == http.StatusNoContent {
		return nil, fmt.Errorf("response status %s: %w", response.Status, CatalogNoContentErr)
	}

	mdefSlice := []marketplacev1beta1.MeterDefinition{}

	defer response.Body.Close()

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		reqLogger.Error(err, "error reading body")
		return nil, err
	}

	reqLogger.Info("response data", "data", string(responseData))

	err = yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(responseData)), 100).Decode(&mdefSlice)
	if err != nil {
		reqLogger.Error(err, "error decoding response from ListMeterdefinitions()")
		return nil, err
	}

	return mdefSlice, nil
}

func (c *CatalogClient) GetSystemMeterdefs(csv *olmv1alpha1.ClusterServiceVersion, reqLogger logr.Logger) ([]marketplacev1beta1.MeterDefinition, error) {

	reqLogger.Info("retrieving system meterdefinitions", "csvName", csv.Name)

	url, err := concatPaths(c.Endpoint.String(), GetSystemMeterdefinitionTemplatesEndpoint)
	if err != nil {
		return nil, err
	}

	requestBody, err := json.Marshal(csv)
	if err != nil {
		return nil, err
	}

	reqLogger.Info("call system meterdef endpoint", "url", url.String())

	response, err := c.HttpClient.Post(url.String(),
		"application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		reqLogger.Error(err, "Error querying file server for system meter definition")
		return nil, err
	}
	
	if response.StatusCode != http.StatusOK {
		if response.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("response status %s: %w", response.Status, CatalogUnauthorizedErr)
		}
	
		if response.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("response status %s: %w", response.Status, CatalogPathNotFoundStatus)
		}
	
		if response.StatusCode == http.StatusNoContent {
			return nil, fmt.Errorf("response status %s: %w", response.Status, CatalogNoContentErr)
		}

		return nil,errors.New(fmt.Sprintf("Error querying file server for system meter definition: %s:%d",response.Status,response.StatusCode))
	}

	defer response.Body.Close()

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		reqLogger.Error(err, "error reading body")
		return nil, err
	}

	reqLogger.Info("response data from GetSystemMeterdefinitions()", "data", string(responseData))

	mdefSlice := []marketplacev1beta1.MeterDefinition{}

	err = yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(responseData)), 100).Decode(&mdefSlice)
	if err != nil {
		reqLogger.Error(err, "error decoding response from GetSystemMeterdefinitions()")
		return nil, err
	}

	return mdefSlice, nil
}

func (c *CatalogClient) GetMeterdefIndexLabels(reqLogger logr.Logger, csvName string) (map[string]string, error) {
	reqLogger.Info("retrieving meterdefinition index label")

	url, err := concatPaths(c.Endpoint.String(), GetMeterdefinitionIndexLabelEndpoint, csvName)
	if err != nil {
		return nil, err
	}

	reqLogger.Info("calling file server for meterdef index labels", "url", url.String())

	response, err := c.HttpClient.Get(url.String())
	if err != nil {
		return nil, err
	}

	if response.StatusCode != 200 {

		if response.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("response status %s: %w", response.Status, CatalogUnauthorizedErr)
		}

		return nil,errors.New(fmt.Sprintf("Error querying file server for meterdefinition index labels: %s:%d",response.Status,response.StatusCode))
	}

	defer response.Body.Close()

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		reqLogger.Error(err, "error reading body")
		return nil, err
	}

	reqLogger.Info("response data", "data", string(data))

	labels := map[string]string{}
	err = json.Unmarshal(data, &labels)
	if err != nil {
		return nil, err
	}

	return labels, nil
}

func (c *CatalogClient) GetSystemMeterDefIndexLabels(reqLogger logr.Logger, csvName string) (map[string]string, error) {
	reqLogger.Info("retrieving system meterdefinition index label")

	url, err := concatPaths(c.Endpoint.String(), GetSystemMeterDefIndexLabelEndpoint, csvName)
	if err != nil {
		return nil, err
	}

	reqLogger.Info("calling file server for system meterdef index labels", "url", url.String())

	response, err := c.HttpClient.Get(url.String())
	if err != nil {
		return nil, err
	}

	if response.StatusCode != 200 {

		if response.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("response status %s: %w", response.Status, CatalogUnauthorizedErr)
		}

		return nil,errors.New(fmt.Sprintf("Error querying file server for meterdefinition index labels: %s:%d",response.Status,response.StatusCode))
	}

	defer response.Body.Close()

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		reqLogger.Error(err, "error reading body")
		return nil, err
	}

	reqLogger.Info("response data", "data", string(data))

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

func (c *CatalogClient) getCatalogServerService(reqLogger logr.InfoLogger) (*corev1.Service, error) {
	service := &corev1.Service{}

	err := c.K8sClient.Get(context.TODO(), types.NamespacedName{Namespace: c.DeployedNamespace, Name: utils.CATALOG_SERVER_SERVICE_NAME}, service)
	if err != nil {
		return nil, err
	}

	return service, nil
}

func (c *CatalogClient) getCertFromConfigMap(reqLogger logr.Logger) ([]byte, error) {
	cm := &corev1.ConfigMap{}
	err := c.K8sClient.Get(context.TODO(), types.NamespacedName{Namespace: c.DeployedNamespace, Name: "serving-certs-ca-bundle"}, cm)
	if err != nil {
		return nil, err
	}

	reqLogger.Info("extracting cert from config map")

	out, ok := cm.Data["service-ca.crt"]

	if !ok {
		err = emperror.New("Error retrieving cert from config map")
		return nil, err
	}

	cert := []byte(out)
	return cert, nil

}

func WithBearerAuth(rt http.RoundTripper, token string) http.RoundTripper {
	addHead := WithHeader(rt)
	addHead.Header.Set("Authorization", "Bearer "+token)
	return addHead
}

type withHeader struct {
	http.Header
	rt http.RoundTripper
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
