package catalog

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
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
	GetSystemMeterDefIndexLabelEndpoint       = "system-meterdef-index-label"
	HealthEndpoint                            = "healthz"
)

var (
	CatalogNoContentErr       error = errors.New("not found")
	CatalogPathNotFoundStatus error = errors.New("the path to the file server wasn't found")
	CatalogUnauthorizedErr    error = errors.New("auth error calling file server")
	defaultRetryWaitMin             = 1 * time.Second
	defaultRetryWaitMax             = 30 * time.Second
	defaultRetryMax                 = 4
)

type CatalogClient struct {
	sync.Mutex
	Endpoint          *url.URL
	HttpClient        *http.Client
	K8sClient         client.Client
	KubeInterface     kubernetes.Interface
	DeployedNamespace string
	RetryWaitMin      time.Duration
	RetryWaitMax      time.Duration
	RetryMax          int
}

type Request struct {
	body io.ReadSeeker
	*http.Request
}

func NewRequest(method string, url string, body io.ReadSeeker) (*Request, error) {
	var rcBody io.ReadCloser
	if body != nil {
		rcBody = ioutil.NopCloser(body)
	}

	httpReq, err := http.NewRequest(method, url, rcBody)
	if err != nil {
		return nil, err
	}

	return &Request{body, httpReq}, nil
}

func (c *CatalogClient) RetryOnAuthError(resp *http.Response, err error, reqLogger logr.Logger) (bool, error) {
	if err != nil {
		return true, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return true, nil
	}

	return false, nil

}

func (c *CatalogClient) DefaultBackoff(min, max time.Duration, attemptNum int, resp *http.Response) time.Duration {
	mult := math.Pow(2, float64(attemptNum)) * float64(min)
	sleep := time.Duration(mult)
	if float64(sleep) != mult || sleep > max {
		sleep = max
	}

	return sleep
}

func (c *CatalogClient) Do(req *Request, reqLogger logr.Logger) (*http.Response, error) {
	reqLogger.Info("calling", "method", req.Method, "url", req.URL)

	if c.HttpClient == nil {
		reqLogger.Info("setting transport on catalog client")

		err := c.SetTransport(reqLogger)
		if err != nil {
			err = errors.Wrap(err, "error setting transport on Catalog Client")
			return nil, err
		}
	}

	retryCount := 0

	for retryCount < c.RetryMax {
		var code int

		//rewind the request if body is non-nil
		if req.body != nil {
			if _, err := req.body.Seek(0, 0); err != nil {
				return nil, fmt.Errorf("failed to seek body: %v", err)
			}
		}

		resp, err := c.HttpClient.Do(req.Request)

		retry, checkErr := c.RetryOnAuthError(resp, err, reqLogger)

		if err != nil {
			return nil, fmt.Errorf("%s %s request failed: %v", req.Method, req.URL, err)
		} else {

		}

		if !retry {
			if checkErr != nil {
				err = checkErr
			}

			return resp, err
		}

		if err == nil {
			c.drainbody(resp.Body)
		}

		// check auth
		reqLogger.Info("checking auth inside Do() wrapper")
		err = c.SetTransport(reqLogger)
		if err != nil {
			err = errors.Wrap(err, "error setting transport on Catalog Client")
			return nil, err
		}

		remaining := c.RetryMax - retryCount
		if remaining == 0 {
			break
		}

		wait := c.DefaultBackoff(c.RetryWaitMin, c.RetryWaitMax, retryCount, resp)
		desc := fmt.Sprintf("%s %s", req.Method, req.URL)
		if code > 0 {
			desc = fmt.Sprintf("%s (status: %d)", desc, code)
		}

		reqLogger.Info("retrying call", "call description", desc, "time until next", wait, "retries remaining", remaining)
		time.Sleep(wait)
		retryCount++
	}

	return nil, fmt.Errorf("%s %s terminating call after %d attempts", req.Method, req.URL, c.RetryMax+1)
}

func (c *CatalogClient) drainbody(body io.ReadCloser) error {
	defer body.Close()
	_, err := io.Copy(ioutil.Discard, io.LimitReader(body, 100))
	if err != nil {
		return fmt.Errorf("error reading response body in drainBody: %v", err)
	}

	return nil
}

func (c *CatalogClient) Get(url string, reqLogger logr.Logger) (*http.Response, error) {
	req, err := NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	return c.Do(req, reqLogger)
}

func (c *CatalogClient) Post(url string, body io.ReadSeeker, reqLogger logr.Logger) (*http.Response, error) {
	req, err := NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	return c.Do(req, reqLogger)
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
		RetryWaitMin:      defaultRetryWaitMin,
		RetryWaitMax:      defaultRetryWaitMax,
		RetryMax:          defaultRetryMax,
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

func (c *CatalogClient) ListMeterdefintionsFromFileServer(csvName string, version string, namespace string, reqLogger logr.Logger) ([]marketplacev1beta1.MeterDefinition, error) {
	reqLogger.Info("retrieving community meterdefinitions", "csvName", csvName, "csvVersion", version)

	url, err := concatPaths(c.Endpoint.String(), ListForVersionEndpoint, csvName, version, namespace)
	if err != nil {
		return nil, err
	}

	response, err := c.Get(url.String(), reqLogger)
	if err != nil {
		reqLogger.Error(err, "Error on GET to Catalog Server")
		return nil, err
	}

	if response.StatusCode != http.StatusOK {

		if response.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("response status %s: %w", response.Status, CatalogPathNotFoundStatus)
		}

		if response.StatusCode == http.StatusNoContent {
			return nil, fmt.Errorf("response status %s: %w", response.Status, CatalogNoContentErr)
		}

		return nil, errors.New(fmt.Sprintf("Error querying file server for community meter definition: %s:%d", response.Status, response.StatusCode))

	}

	mdefSlice := []marketplacev1beta1.MeterDefinition{}

	defer response.Body.Close()

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		reqLogger.Error(err, "error reading body")
		return nil, err
	}

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

	r := bytes.NewReader(requestBody)
	response, err := c.Post(url.String(), r, reqLogger)
	if err != nil {
		reqLogger.Error(err, "Error querying file server for system meter definition")
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
		reqLogger.Error(err, "error reading body")
		return nil, err
	}

	mdefSlice := []marketplacev1beta1.MeterDefinition{}

	err = yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(responseData)), 100).Decode(&mdefSlice)
	if err != nil {
		reqLogger.Error(err, "error decoding response from GetSystemMeterdefinitions()")
		return nil, err
	}

	return mdefSlice, nil
}

func (c *CatalogClient) GetCommunityMeterdefIndexLabels(reqLogger logr.Logger, csvName string) (map[string]string, error) {
	reqLogger.Info("retrieving community meterdefinition index label")

	url, err := concatPaths(c.Endpoint.String(), GetMeterdefinitionIndexLabelEndpoint, csvName)
	if err != nil {
		return nil, err
	}

	response, err := c.Get(url.String(), reqLogger)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("Error querying file server for meterdefinition index labels: %s:%d", response.Status, response.StatusCode))
	}

	defer response.Body.Close()

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		reqLogger.Error(err, "error reading body")
		return nil, err
	}

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

	response, err := c.Get(url.String(), reqLogger)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("Error querying file server for meterdefinition index labels: %s:%d", response.Status, response.StatusCode))
	}

	defer response.Body.Close()

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		reqLogger.Error(err, "error reading body")
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

func (c *CatalogClient) getCatalogServerService(reqLogger logr.InfoLogger) (*corev1.Service, error) {
	service := &corev1.Service{}

	err := c.K8sClient.Get(context.TODO(), types.NamespacedName{Namespace: c.DeployedNamespace, Name: utils.DeploymentConfigName}, service)
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
