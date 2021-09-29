package transport

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
	"time"

	emperror "emperror.dev/errors"
	"github.com/go-logr/logr"
	prom "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/prometheus"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func SetTransportOnHTTPClient(k8sclient client.Client, deployedNamespace string, ki kubernetes.Interface, reqLogger logr.Logger) (*http.Client, error) {
	service, err := getCatalogServerService(k8sclient, deployedNamespace)
	if err != nil {
		return nil, err
	}

	cert, err := getCertFromConfigMap(k8sclient, deployedNamespace, reqLogger)
	if err != nil {
		return nil, err
	}

	saClient := prom.NewServiceAccountClient(deployedNamespace, ki)
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

		catalogServerHTTPClient := &http.Client{
			Transport: transport,
			Timeout:   1 * time.Second,
		}

		return catalogServerHTTPClient, nil
	}

	err = errors.New("could not construct http client with transport")
	return nil, err
}

func getCatalogServerService(k8sclient client.Client, deployedNamespace string) (*corev1.Service, error) {
	service := &corev1.Service{}

	err := k8sclient.Get(context.TODO(), types.NamespacedName{Namespace: deployedNamespace, Name: utils.DeploymentConfigName}, service)
	if err != nil {
		return nil, err
	}

	return service, nil
}

func getCertFromConfigMap(k8sclient client.Client, deployedNamespace string, reqLogger logr.Logger) ([]byte, error) {
	cm := &corev1.ConfigMap{}
	err := k8sclient.Get(context.TODO(), types.NamespacedName{Namespace: deployedNamespace, Name: "serving-certs-ca-bundle"}, cm)
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
