package rhmotransport

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
	"time"

	emperror "emperror.dev/errors"
	"github.com/go-logr/logr"

	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/client-go/kubernetes"
)

type IAuthBuilder interface {
	FindAuthOffCluster() (*AuthValues, error)
}

type AuthBuilderConfig struct {
	K8sclient         client.Client
	DeployedNamespace string
	*ServiceAccountClient
	Logger logr.Logger
	*AuthValues
	Error error
}

type AuthValues struct {
	ServiceFound bool
	Cert         []byte
	AuthToken    string
}

func ProvideAuthBuilder(k8sclient client.Client, operatorConfig *config.OperatorConfig, kubeInterface kubernetes.Interface, reqLogger logr.Logger) *AuthBuilderConfig {
	saClient := ProvideServiceAccountClient(operatorConfig.DeployedNamespace, kubeInterface)

	return &AuthBuilderConfig{
		K8sclient:            k8sclient,
		DeployedNamespace:    operatorConfig.DeployedNamespace,
		ServiceAccountClient: saClient,
		Logger:               reqLogger,
	}
}

func (a *AuthBuilderConfig) FindAuthOffCluster() (*AuthValues, error) {
	_, err := GetCatalogServerService(a.K8sclient, a.DeployedNamespace)
	if err != nil {
		return nil, err
	}

	cert, err := GetCertFromConfigMap(a.K8sclient, a.DeployedNamespace, a.Logger)
	if err != nil {
		return nil, err
	}

	authToken, err := a.NewServiceAccountToken(utils.OPERATOR_SERVICE_ACCOUNT, utils.FileServerAudience, 3600, a.Logger)
	if err != nil {
		return nil, err
	}

	return &AuthValues{
		ServiceFound: true,
		Cert:         cert,
		AuthToken:    authToken,
	}, nil
}

func SetTransportForKubeServiceAuth(authBuilder IAuthBuilder, reqLogger logr.Logger) (*http.Client, error) {
	transportAuth, err := authBuilder.FindAuthOffCluster()
	if err != nil {
		return nil, err
	}

	if transportAuth.ServiceFound && len(transportAuth.Cert) != 0 && transportAuth.AuthToken != "" {
		caCertPool, err := x509.SystemCertPool()
		if err != nil {
			return nil, err
		}

		ok := caCertPool.AppendCertsFromPEM(transportAuth.Cert)
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

		transport = WithBearerAuth(transport, transportAuth.AuthToken)

		catalogServerHTTPClient := &http.Client{
			Transport: transport,
			Timeout:   1 * time.Second,
		}

		return catalogServerHTTPClient, nil
	}

	err = errors.New("could not construct http client with transport, all auth fields are not set on AuthBuilderConfig")
	return nil, err
}

func GetCatalogServerService(k8sclient client.Client, deployedNamespace string) (*corev1.Service, error) {
	service := &corev1.Service{}

	err := k8sclient.Get(context.TODO(), types.NamespacedName{Namespace: deployedNamespace, Name: utils.DeploymentConfigName}, service)
	if err != nil {
		return nil, err
	}

	return service, nil
}

func GetCertFromConfigMap(k8sclient client.Client, deployedNamespace string, reqLogger logr.Logger) ([]byte, error) {
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

type TransportWithHeader struct {
	http.Header
	rt http.RoundTripper
}

func WithHeader(rt http.RoundTripper) TransportWithHeader {
	if rt == nil {
		rt = http.DefaultTransport
	}

	return TransportWithHeader{Header: make(http.Header), rt: rt}
}

func (m TransportWithHeader) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range m.Header {
		req.Header[k] = v
	}

	return m.rt.RoundTrip(req)
}
