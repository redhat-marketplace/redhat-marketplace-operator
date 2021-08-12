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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	FileServerProductionURL = "https://rhm-meterdefinition-file-server.openshift-redhat-marketplace.svc.cluster.local:8200"
	ListForVersionEndpoint = "list-for-version"
	GetSystemMeterdefinitionTemplatesEndpoint = "get-system-meterdefs"
	GetMeterdefinitionIndexLabelEndpoint = "meterdef-index-label"
)

type CatalogResponseStatusType string

/* 
	44-61 are temporary - we can import these types from the file server repo 
*/
const (
	CsvDoesNotHaveCatalogDirStatus = "csv does not have a directory in the catalog"
	CsvHasNoMeterdefinitionsStatus = "does not have meterdefinitions in csv directory"
	CsvWithMeterdefsFoundStatus    = "csv has meterdefinitions listed in the community catalog"
)

type CatalogStatus struct {
	StatusMessage string `json:"statusMessage,omitempty"`

	//TODO: need this ? if the client gets a CatalogStatus it's a 200 anyways
	HttpStatus       int                       `json:"httpStatus,omitempty"`
	CatlogStatusType CatalogResponseStatusType `json:"catalogStatusType,omitempty"`
}

type CatalogResponse struct {
	CatalogStatus *CatalogStatus            `json:"requestError,omitempty"`
	MdefList      []marketplacev1beta1.MeterDefinition `json:"mdefList,omitempty"`
}

type CatalogClientBuilder struct {
	Url      string
}

type CatalogClient struct {
	endpoint   *url.URL
	httpClient http.Client
}

func NewCatalogClientBuilder(cfg *config.OperatorConfig) *CatalogClientBuilder {
	builder := &CatalogClientBuilder{}

	builder.Url = FileServerProductionURL

	if cfg.FileServerURL != "" {
		builder.Url = cfg.FileServerURL
	}

	return builder
}

func (b *CatalogClientBuilder) NewCatalogServerClient(client client.Client, deployedNamespace string, kubeInterface kubernetes.Interface, reqLogger logr.Logger) (*CatalogClient, error) {
	service, err := getCatalogServerService(deployedNamespace, client, reqLogger)
	if err != nil {
		return nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	cert, err := getCertFromConfigMap(client, deployedNamespace, reqLogger)
	if err != nil {
		return nil, err
	}

	saClient := prom.NewServiceAccountClient(deployedNamespace, kubeInterface)
	authToken, err := saClient.NewServiceAccountToken(utils.OPERATOR_SERVICE_ACCOUNT, utils.FileServerAudience, 3600, reqLogger)
	if err != nil {
		return nil, err
	}

	if service != nil && len(cert) != 0 && authToken != "" {
		caCertPool, err := x509.SystemCertPool()
		if err != nil {
			return nil, err
		}

		ok := caCertPool.AppendCertsFromPEM(cert)
		if !ok {
			err = emperror.New("failed to append cert to cert pool")
			reqLogger.Error(err, "cert pool error")
			return nil, err
		}

		tlsConfig := &tls.Config{
			RootCAs: caCertPool,
		}

		var transport http.RoundTripper = &http.Transport{
			TLSClientConfig: tlsConfig,
			Proxy:           http.ProxyFromEnvironment,
		}

		transport = WithBearerAuth(transport, authToken)

		catalogServerClient := http.Client{
			Transport: transport,
			Timeout:   1 * time.Second,
		}

		url,err := url.Parse(b.Url)
		if err != nil {
			return nil,err
		}

		reqLogger.Info("Catalog Server client created successfully")
		return &CatalogClient{
			httpClient: catalogServerClient,
			endpoint: url,
		}, nil
	}

	return nil, &ExecResult{
		ReconcileResult: reconcile.Result{},
		Err:             emperror.New("catalog server client prerequisites not ready"),
	}
}

func(c *CatalogClient) ListMeterdefintionsFromFileServer(csvName string, version string, namespace string,reqLogger logr.Logger) (*CatalogResponse, *ExecResult) {
	reqLogger.Info("retrieving meterdefinitions", "csvName", csvName, "csvVersion", version)

	url,err := concatPaths(c.endpoint.String(),ListForVersionEndpoint,csvName,version)
	if err != nil {
		return nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	response, err := c.httpClient.Get(url.String())
	if err != nil {
		reqLogger.Error(err, "Error on GET to Catalog Server")

		reqLogger.Error(err, "Error querying file server")
		return nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	catalogResponse := &CatalogResponse{}

	defer response.Body.Close()

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		reqLogger.Error(err, "error reading body")
		return nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	reqLogger.Info("response data", "data", string(responseData))

	err = yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(responseData)), 100).Decode(&catalogResponse)
	if err != nil {
		reqLogger.Error(err, "error decoding response from fetchGlobalMeterdefinitions()")
		return nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}


	return catalogResponse,&ExecResult{
		Status: ActionResultStatus(Continue),
	}
}

func  ReturnMeterdefs (mdefSlice []marketplacev1beta1.MeterDefinition,csvName string, namespace string,reqLogger logr.Logger) ([]marketplacev1beta1.MeterDefinition, *ExecResult){

	var out []marketplacev1beta1.MeterDefinition
	
	for _, meterDefItem := range mdefSlice {
		meterDefItem.Namespace = namespace
		out = append(out,meterDefItem)
	}

	return out, &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}

func (c *CatalogClient) GetSystemMeterdefs(csvName string, version string, namespace string, reqLogger logr.Logger) ([]string, []marketplacev1beta1.MeterDefinition, *ExecResult) {

	reqLogger.Info("retrieving system meterdefinitions", "csvName", csvName, "csvVersion", version)

	url,err := concatPaths(c.endpoint.String(),GetSystemMeterdefinitionTemplatesEndpoint,csvName,version,namespace)
	if err != nil {
		return nil,nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	response, err := c.httpClient.Get(url.String())
	if err != nil {
		reqLogger.Error(err, "Error querying file server")
		return nil, nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	return ReturnSystemMeterdefs(csvName,namespace,*response,reqLogger)

}

func  ReturnSystemMeterdefs (csvName string, namespace string,response http.Response,reqLogger logr.Logger) ([]string, []marketplacev1beta1.MeterDefinition, *ExecResult){
	meterDefNames := []string{}
	mdefSlice := []marketplacev1beta1.MeterDefinition{}

	defer response.Body.Close()

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		reqLogger.Error(err, "error reading body")
		return nil, nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	if response.StatusCode == 404 {
		reqLogger.Info(response.Status)
		err = emperror.New(response.Status)
		return nil, nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	reqLogger.Info("response data", "data", string(data))

	meterDefsData := strings.Replace(string(data), "<<NAMESPACE-PLACEHOLDER>>", namespace, -1)
	err = yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(meterDefsData)), 100).Decode(&mdefSlice)
	if err != nil {
		reqLogger.Error(err, "error decoding response from fetchGlobalMeterdefinitions()")
		return nil, nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	for _, meterDefItem := range mdefSlice {
		meterDefNames = append(meterDefNames, meterDefItem.ObjectMeta.Name)
	}

	reqLogger.Info("meterdefintions returned from file server", csvName, meterDefNames)

	return meterDefNames, mdefSlice, &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}

func (c *CatalogClient) GetMeterdefIndexLabels (reqLogger logr.Logger,csvName string) (map[string]string,*ExecResult) {
	reqLogger.Info("retrieving meterdefinition index label")

	url,err := concatPaths(c.endpoint.String(),GetMeterdefinitionIndexLabelEndpoint,csvName)
	if err != nil {
		return nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	reqLogger.Info("calling file server for meterdef index labels","url",url.String())

	response, err := c.httpClient.Get(url.String())
	if err != nil {
		return nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	defer response.Body.Close()

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		reqLogger.Error(err, "error reading body")
		return nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}

	reqLogger.Info("response data", "data", string(data))

	labels := map[string]string{}
	err = json.Unmarshal(data,&labels)
	if err != nil {
		return nil, &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}
	
	return labels, &ExecResult{
		Status: ActionResultStatus(Continue),
	}
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
