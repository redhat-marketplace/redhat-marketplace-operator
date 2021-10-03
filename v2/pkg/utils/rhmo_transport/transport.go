package rhmo_transport

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
	"time"

	"fmt"
	"sync"

	emperror "emperror.dev/errors"
	"github.com/go-logr/logr"

	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gotidy/ptr"
	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type IAuthBuilder interface {
	FindAuthOffCluster() (*AuthValues, error)
}

type AuthBuilderConfig struct {
	K8sclient         client.Client
	DeployedNamespace string
	KubeInterface     kubernetes.Interface
	Logger            logr.Logger
	*AuthValues
	Error error
}

type AuthValues struct {
	ServiceFound bool
	Cert         []byte
	AuthToken    string
}

type ServiceAccountClient struct {
	KubernetesInterface kubernetes.Interface
	Token               *Token
	TokenRequestObj     *authv1.TokenRequest
	Client              typedv1.ServiceAccountInterface
	sync.Mutex
}

type Token struct {
	AuthToken           *string
	ExpirationTimestamp metav1.Time
}

func ProvideAuthBuilder(k8sclient client.Client, operatorConfig *config.OperatorConfig, kubeInterface kubernetes.Interface, reqLogger logr.Logger) *AuthBuilderConfig {
	return &AuthBuilderConfig{
		K8sclient:         k8sclient,
		DeployedNamespace: operatorConfig.DeployedNamespace,
		KubeInterface:     kubeInterface,
		Logger:            reqLogger,
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

	saClient := NewServiceAccountClient(a.DeployedNamespace, a.KubeInterface)
	authToken, err := saClient.NewServiceAccountToken(utils.OPERATOR_SERVICE_ACCOUNT, utils.FileServerAudience, 3600, a.Logger)
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

func NewServiceAccountClient(namespace string, kubernetesInterface kubernetes.Interface) *ServiceAccountClient {
	return &ServiceAccountClient{
		Client: kubernetesInterface.CoreV1().ServiceAccounts(namespace),
	}
}

func (s *ServiceAccountClient) NewServiceAccountToken(targetServiceAccountName string, audience string, expireSecs int64, reqLogger logr.Logger) (string, error) {
	s.Lock()
	defer s.Unlock()

	now := metav1.Now().UTC()
	opts := metav1.CreateOptions{}
	tr := s.newTokenRequest(audience, expireSecs)

	if s.Token == nil {
		reqLogger.Info("auth token from service account not found")

		return s.GetToken(targetServiceAccountName, s.Client, tr, opts)
	}

	if now.UTC().After(s.Token.ExpirationTimestamp.Time) {

		reqLogger.Info("service account token is expired")

		return s.GetToken(targetServiceAccountName, s.Client, tr, opts)
	}

	return s.GetToken(targetServiceAccountName, s.Client, tr, opts)
}

func (s *ServiceAccountClient) newTokenRequest(audience string, expireSeconds int64) *authv1.TokenRequest {
	if len(audience) != 0 {
		return &authv1.TokenRequest{
			Spec: authv1.TokenRequestSpec{
				Audiences:         []string{audience},
				ExpirationSeconds: ptr.Int64(expireSeconds),
			},
		}
	} else {
		return &authv1.TokenRequest{
			Spec: authv1.TokenRequestSpec{
				ExpirationSeconds: ptr.Int64(expireSeconds),
			},
		}
	}
}

func (s *ServiceAccountClient) GetToken(targetServiceAccount string, client typedv1.ServiceAccountInterface, tr *authv1.TokenRequest, opts metav1.CreateOptions) (string, error) {
	tr, err := client.CreateToken(context.TODO(), targetServiceAccount, tr, opts)
	if err != nil {
		e := fmt.Sprintf("create token error %s", err)
		return "", errors.New(e)
	}

	s.Token = &Token{
		AuthToken:           ptr.String(tr.Status.Token),
		ExpirationTimestamp: tr.Status.ExpirationTimestamp,
	}

	token := tr.Status.Token
	return token, nil
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
