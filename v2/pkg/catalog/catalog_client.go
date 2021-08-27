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
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	FileServerProductionURL                   = "https://rhm-meterdefinition-file-server.openshift-redhat-marketplace.svc.cluster.local:8200"
	ListForVersionEndpoint                    = "list-for-version"
	GetSystemMeterdefinitionTemplatesEndpoint = "get-system-meterdefs"
	GetMeterdefinitionIndexLabelEndpoint      = "meterdef-index-label"
)

type CatalogResponseStatusType string

const (
	CsvHasNoMeterdefinitionsStatus CatalogResponseStatusType = "does not have meterdefinitions in csv directory"
	CsvWithMeterdefsFoundStatus    CatalogResponseStatusType = "csv has meterdefinitions listed in the community catalog"
	CatalogPathNotFoundStatus      CatalogResponseStatusType = "the path to the file server wasn't found"
	UnauthorizedStatus             CatalogResponseStatusType = "auth error calling file server"
	SystemMeterdefsReturnedStatus  CatalogResponseStatusType = "successfully returned system meter definitions"
)

type CatalogStatus struct {
	StatusCode       int
	Status           string
	CsvName          string
	CatlogStatusType CatalogResponseStatusType
}

type CatalogResponse struct {
	*CatalogStatus
	MdefSlice   []marketplacev1beta1.MeterDefinition
	IndexLabels map[string]string
}

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

func (c *CatalogClient) ListMeterdefintionsFromFileServer(csvName string, version string, namespace string, reqLogger logr.Logger) (*CatalogResponse, error) {
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
		return &CatalogResponse{
			CatalogStatus: &CatalogStatus{
				StatusCode:       response.StatusCode,
				Status:           response.Status,
				CatlogStatusType: UnauthorizedStatus,
				CsvName:          csvName,
			},
		}, nil
	}

	if response.StatusCode == http.StatusNotFound {
		return nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             errors.New(fmt.Sprintf("not found %d", response.StatusCode)),
		}
	}

	if response.StatusCode == http.StatusNoContent {
		return &CatalogResponse{
			CatalogStatus: &CatalogStatus{
				StatusCode:       response.StatusCode,
				CatlogStatusType: CsvHasNoMeterdefinitionsStatus,
				CsvName:          csvName,
			},
		}, nil
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

	return &CatalogResponse{
		CatalogStatus: &CatalogStatus{
			StatusCode:       response.StatusCode,
			CatlogStatusType: CsvWithMeterdefsFoundStatus,
			CsvName:          csvName,
		},
		MdefSlice: mdefSlice,
	}, nil
}

func (c *CatalogClient) GetSystemMeterdefs(csv *olmv1alpha1.ClusterServiceVersion, reqLogger logr.Logger) (*CatalogResponse, error) {

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

	if response.StatusCode == http.StatusUnauthorized {
		reqLogger.Info("auth error,setting transport and requeueing", "response status", response.Status)
		return &CatalogResponse{
			CatalogStatus: &CatalogStatus{
				StatusCode:       response.StatusCode,
				Status:           response.Status,
				CatlogStatusType: UnauthorizedStatus,
			},
		}, nil
	}

	if response.StatusCode == http.StatusNotFound {
		return nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             errors.New(fmt.Sprintf("not found %d", response.StatusCode)),
		}
	}

	if response.StatusCode == http.StatusNoContent {
		return &CatalogResponse{
			CatalogStatus: &CatalogStatus{
				StatusCode:       response.StatusCode,
				CatlogStatusType: CsvHasNoMeterdefinitionsStatus,
			},
		}, nil
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

	return &CatalogResponse{
		CatalogStatus: &CatalogStatus{
			StatusCode:       response.StatusCode,
			CatlogStatusType: SystemMeterdefsReturnedStatus,
			CsvName:          csv.Name,
		},
		MdefSlice: mdefSlice,
	}, nil
}

func (c *CatalogClient) GetMeterdefIndexLabels(reqLogger logr.Logger, csvName string) (*CatalogResponse, error) {
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

	if response.StatusCode == http.StatusUnauthorized {
		return &CatalogResponse{
			CatalogStatus: &CatalogStatus{
				StatusCode:       response.StatusCode,
				Status:           response.Status,
				CatlogStatusType: UnauthorizedStatus,
			},
		}, nil
	}

	if response.StatusCode == http.StatusNotFound {
		return nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             errors.New(fmt.Sprintf("not found %s", response.Status)),
		}
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

	return &CatalogResponse{
		CatalogStatus: &CatalogStatus{
			StatusCode:       response.StatusCode,
			CatlogStatusType: SystemMeterdefsReturnedStatus,
		},
		IndexLabels: labels,
	}, nil
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
