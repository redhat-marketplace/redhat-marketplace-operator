package catalog

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	emperror "emperror.dev/errors"
	"github.com/go-logr/logr"

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
)

const (
	FileServerProductionURL = "https://rhm-meterdefinition-file-server.openshift-redhat-marketplace.svc.cluster.local:8200"
	ListForVersionEndpoint = "list-for-version"
	GetSystemMeterdefinitionTemplatesEndpoint = "get-system-meterdefs"
	GetMeterdefinitionIndexLabelEndpoint = "meterdef-index-label"
)

type CatalogResponseStatusType string

const (
	CsvHasNoMeterdefinitionsStatus CatalogResponseStatusType = "does not have meterdefinitions in csv directory"
	CsvWithMeterdefsFoundStatus CatalogResponseStatusType = "csv has meterdefinitions listed in the community catalog"
	CatalogPathNotFoundStatus CatalogResponseStatusType = "the path to the file server wasn't found"
)

type CatalogStatus struct {
	StatusCode int
	CsvName string
	CatlogStatusType CatalogResponseStatusType 
}

type CatalogResponse struct {
	*CatalogStatus
	MdefSlice      []marketplacev1beta1.MeterDefinition 
}

type CatalogClient struct {
	endpoint   *url.URL
	httpClient http.Client
}

func ProvideCatalogClient(cfg *config.OperatorConfig)(*CatalogClient,error){
	fileServerUrl := FileServerProductionURL

		if cfg.FileServerURL != "" {
			fileServerUrl = cfg.FileServerURL
		}

		url,err := url.Parse(fileServerUrl)
		if err != nil {
			return nil,err
		}

		return &CatalogClient{
			endpoint: url,
		}, nil

}

func(c *CatalogClient) SetTransport (client client.Client,cfg *config.OperatorConfig,kubeInterface kubernetes.Interface,reqLogger logr.Logger)error{
	service, err := getCatalogServerService(cfg.DeployedNamespace, client, reqLogger)
	if err != nil {
		return err
	}

	cert, err := getCertFromConfigMap(client, cfg.DeployedNamespace, reqLogger)
	if err != nil {
		return err
	}

	saClient := prom.NewServiceAccountClient(cfg.DeployedNamespace, kubeInterface)
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

		catalogServerHttpClient := http.Client{
			Transport: transport,
			Timeout:   1 * time.Second,
		}

		c.httpClient = catalogServerHttpClient
	}

	return nil
}

func(c *CatalogClient) ListMeterdefintionsFromFileServer(csvName string, version string, namespace string,reqLogger logr.Logger) (*CatalogResponse, error) {
	reqLogger.Info("retrieving meterdefinitions", "csvName", csvName, "csvVersion", version)

	url,err := concatPaths(c.endpoint.String(),ListForVersionEndpoint,csvName,version)
	if err != nil {
		return nil, err
	}

	response, err := c.httpClient.Get(url.String())
	if err != nil {
		reqLogger.Error(err, "Error on GET to Catalog Server")
		return nil, err
	}

	if response.StatusCode == http.StatusNotFound {
		return &CatalogResponse{
			CatalogStatus: &CatalogStatus{
				StatusCode: response.StatusCode,
				CatlogStatusType: CatalogPathNotFoundStatus,
				CsvName: csvName,
			},
		},nil
	}

	if response.StatusCode == http.StatusNoContent {
		return &CatalogResponse{
			CatalogStatus: &CatalogStatus{
				StatusCode: response.StatusCode,
				CatlogStatusType: CsvHasNoMeterdefinitionsStatus,
				CsvName: csvName,
			},
		},nil
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
		reqLogger.Error(err, "error decoding response from fetchGlobalMeterdefinitions()")
		return nil, err
	}

	return &CatalogResponse{
		CatalogStatus: &CatalogStatus{
			StatusCode: response.StatusCode,
			CatlogStatusType: CsvWithMeterdefsFoundStatus,
			CsvName: csvName,
		},
		MdefSlice: mdefSlice,
	},nil
}

func ReturnMeterdefs (mdefSlice []marketplacev1beta1.MeterDefinition,csvName string, namespace string,reqLogger logr.Logger) ([]marketplacev1beta1.MeterDefinition, error){

	var out []marketplacev1beta1.MeterDefinition
	
	for _, meterDefItem := range mdefSlice {
		meterDefItem.Namespace = namespace
		out = append(out,meterDefItem)
	}

	return out, nil
}

func (c *CatalogClient) GetSystemMeterdefs(csvName string, version string, namespace string, reqLogger logr.Logger) ([]string, []marketplacev1beta1.MeterDefinition, error) {

	reqLogger.Info("retrieving system meterdefinitions", "csvName", csvName, "csvVersion", version)

	url,err := concatPaths(c.endpoint.String(),GetSystemMeterdefinitionTemplatesEndpoint,csvName,version,namespace)
	if err != nil {
		return nil,nil,err
	}

	response, err := c.httpClient.Get(url.String())
	if err != nil {
		reqLogger.Error(err, "Error querying file server")
		return nil, nil, err
	}

	return ReturnSystemMeterdefs(csvName,namespace,*response,reqLogger)

}

func ReturnSystemMeterdefs (csvName string, namespace string,response http.Response,reqLogger logr.Logger) ([]string, []marketplacev1beta1.MeterDefinition,error){
	meterDefNames := []string{}
	mdefSlice := []marketplacev1beta1.MeterDefinition{}

	defer response.Body.Close()

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		reqLogger.Error(err, "error reading body")
		return nil, nil, err
	}

	if response.StatusCode == 404 {
		reqLogger.Info(response.Status)
		err = emperror.New(response.Status)
		return nil, nil, err
	}

	reqLogger.Info("response data", "data", string(data))

	meterDefsData := strings.Replace(string(data), "<<NAMESPACE-PLACEHOLDER>>", namespace, -1)
	err = yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(meterDefsData)), 100).Decode(&mdefSlice)
	if err != nil {
		reqLogger.Error(err, "error decoding response from fetchGlobalMeterdefinitions()")
		return nil, nil, err
	}

	for _, meterDefItem := range mdefSlice {
		meterDefNames = append(meterDefNames, meterDefItem.ObjectMeta.Name)
	}

	reqLogger.Info("meterdefintions returned from file server", csvName, meterDefNames)

	return meterDefNames, mdefSlice, &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}

func (c *CatalogClient) GetMeterdefIndexLabels (reqLogger logr.Logger,csvName string) (map[string]string,error) {
	reqLogger.Info("retrieving meterdefinition index label")

	url,err := concatPaths(c.endpoint.String(),GetMeterdefinitionIndexLabelEndpoint,csvName)
	if err != nil {
		return nil, err
	}

	reqLogger.Info("calling file server for meterdef index labels","url",url.String())

	response, err := c.httpClient.Get(url.String())
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		reqLogger.Error(err, "error reading body")
		return nil, err
	}

	reqLogger.Info("response data", "data", string(data))

	labels := map[string]string{}
	err = json.Unmarshal(data,&labels)
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


func getCatalogServerService(deployedNamespace string, client client.Client, reqLogger logr.InfoLogger) (*corev1.Service, error) {
	service := &corev1.Service{}

	err := client.Get(context.TODO(), types.NamespacedName{Namespace: deployedNamespace, Name: utils.CATALOG_SERVER_SERVICE_NAME}, service)
	if err != nil {
		return nil, err
	}

	return service, nil
}

func getCertFromConfigMap(client client.Client, deployedNamespace string, reqLogger logr.Logger) ([]byte, error) {
	cm := &corev1.ConfigMap{}
	err := client.Get(context.TODO(), types.NamespacedName{Namespace: deployedNamespace, Name: "serving-certs-ca-bundle"}, cm)
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
