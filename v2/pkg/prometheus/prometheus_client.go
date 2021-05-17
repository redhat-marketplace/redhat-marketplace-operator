// Copyright 2020 IBM Corp.
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

package prometheus

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"

	"emperror.dev/errors"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/log"
	v1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type PrometheusSecureClientConfig struct {
	Address string

	Token string

	UserAuth *UserAuth

	ServerCertFile string

	CaCert *[]byte
}

type UserAuth struct {
	Username, Password string
}

type PrometheusAPISetup struct {
	Report        *v1alpha1.MeterReport
	PromService   *corev1.Service
	CertFilePath  string
	TokenFilePath string
	RunLocal      bool
}

func NewPromAPI(
	promService *corev1.Service,
	caCert *[]byte,
	token string,
) (*PrometheusAPI, error) {
	promAPI, err := providePrometheusAPI(promService, caCert, token)
	if err != nil {
		return nil, err
	}
	prometheusAPI := &PrometheusAPI{promAPI}
	return prometheusAPI, nil
}

func NewPrometheusAPIForReporter(
	setup *PrometheusAPISetup,
) (*PrometheusAPI, error) {
	promAPI, err := providePrometheusAPIForReporter(setup)
	if err != nil {
		return nil, err
	}
	prometheusAPI := &PrometheusAPI{promAPI}
	return prometheusAPI, nil
}

func providePrometheusAPI(
	promService *corev1.Service,
	caCert *[]byte,
	token string,
) (v1.API, error) {

	var port int32
	if promService == nil {
		return nil, errors.New("Prometheus service not defined")
	}

	name := promService.Name
	namespace := promService.Namespace
	targetPort := intstr.FromString("rbac")

	switch {
	case targetPort.Type == intstr.Int:
		port = targetPort.IntVal
	default:
		for _, p := range promService.Spec.Ports {
			if p.Name == targetPort.StrVal {
				port = p.Port
			}
		}
	}

	conf, err := NewSecureClientFromCert(&PrometheusSecureClientConfig{
		Address: fmt.Sprintf("https://%s.%s.svc:%v", name, namespace, port),
		Token:   token,
		CaCert:  caCert,
	})

	if err != nil {
		log.Error(err, "failed to setup NewSecureClient")
		return nil, err
	}

	if conf == nil {
		log.Error(err, "failed to setup NewSecureClient")
		return nil, errors.New("client configuration is nil")
	}

	promAPI := v1.NewAPI(conf)
	// p.promAPI = promAPI
	return promAPI, nil
}

func providePrometheusAPIForReporter(
	setup *PrometheusAPISetup,
) (v1.API, error) {

	if setup.RunLocal {
		client, err := api.NewClient(api.Config{
			Address: "http://127.0.0.1:9090",
		})

		if err != nil {
			return nil, err
		}
		localClient := v1.NewAPI(client)
		return localClient, nil
	}

	var port int32
	name := setup.PromService.Name
	namespace := setup.PromService.Namespace
	targetPort := setup.Report.Spec.PrometheusService.TargetPort

	switch {
	case targetPort.Type == intstr.Int:
		port = targetPort.IntVal
	default:
		for _, p := range setup.PromService.Spec.Ports {
			if p.Name == targetPort.StrVal {
				port = p.Port
			}
		}
	}

	var auth = ""
	if setup.TokenFilePath != "" {
		content, err := ioutil.ReadFile(setup.TokenFilePath)
		if err != nil {
			return nil, err
		}
		auth = fmt.Sprintf(string(content))
	}

	conf, err := NewSecureClient(&PrometheusSecureClientConfig{
		Address:        fmt.Sprintf("https://%s.%s.svc:%v", name, namespace, port),
		ServerCertFile: setup.CertFilePath,
		Token:          auth,
	})

	if err != nil {
		return nil, err
	}

	promAPI := v1.NewAPI(conf)
	return promAPI, nil
}

func GetAuthToken(apiTokenPath string) (token string, returnErr error) {
	content, err := ioutil.ReadFile(apiTokenPath)
	if err != nil {
		return "", err
	}
	token = fmt.Sprintf(string(content))
	return token, nil
}

func GetAuthTokenForKubeAdm() (token string, returnErr error) {
	content, err := ioutil.ReadFile("/etc/kubeadmin/token")
	if err != nil {
		return "", err
	}
	token = fmt.Sprintf(string(content))
	return token, nil
}

func NewSecureClient(config *PrometheusSecureClientConfig) (api.Client, error) {
	tlsConfig, err := GenerateCACertPool(config.ServerCertFile)

	if err != nil {
		return nil, errors.Wrap(err, "failed to get tlsConfig")
	}

	var transport http.RoundTripper

	transport = &http.Transport{
		TLSClientConfig: tlsConfig,
		Proxy:           http.ProxyFromEnvironment,
	}

	if config.UserAuth != nil {
		transport = WithBasicAuth(transport, config.UserAuth.Username, config.UserAuth.Password)
	}

	if config.Token != "" {
		transport = WithBearerAuth(transport, config.Token)
	}

	client, err := api.NewClient(api.Config{
		Address:      config.Address,
		RoundTripper: transport,
	})

	return client, err
}

func NewSecureClientFromCert(config *PrometheusSecureClientConfig) (api.Client, error) {

	tlsConfig, err := generateCACertPoolFromCert(*config.CaCert)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tlsConfig")
	}

	var transport http.RoundTripper

	transport = &http.Transport{
		TLSClientConfig: tlsConfig,
		Proxy:           http.ProxyFromEnvironment,
	}

	if config.UserAuth != nil {
		transport = WithBasicAuth(transport, config.UserAuth.Username, config.UserAuth.Password)
	}

	if config.Token != "" {
		transport = WithBearerAuth(transport, config.Token)
	}

	client, err := api.NewClient(api.Config{
		Address:      config.Address,
		RoundTripper: transport,
	})

	return client, err
}

func generateCACertPoolFromCert(caCert []byte) (*tls.Config, error) {
	caCertPool, err := x509.SystemCertPool()

	if err != nil {
		return nil, errors.Wrap(err, "failed to get system cert pool")
	}

	caCertPool.AppendCertsFromPEM(caCert)

	return &tls.Config{
		RootCAs: caCertPool,
	}, nil
}

func GenerateCACertPool(files ...string) (*tls.Config, error) {
	caCertPool, err := x509.SystemCertPool()

	if err != nil {
		return nil, errors.Wrap(err, "failed to get system cert pool")
	}

	for _, file := range files {
		caCert, err := ioutil.ReadFile(file)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load cert file")
		}
		caCertPool.AppendCertsFromPEM(caCert)
	}

	return &tls.Config{
		RootCAs: caCertPool,
	}, nil
}

type withHeader struct {
	http.Header
	rt http.RoundTripper
}

func WithBasicAuth(rt http.RoundTripper, username, password string) http.RoundTripper {
	addHead := WithHeader(rt)
	addHead.Header.Set("Authorization", "Basic "+basicAuth(username, password))
	return addHead
}

func WithBearerAuth(rt http.RoundTripper, token string) http.RoundTripper {
	addHead := WithHeader(rt)
	addHead.Header.Set("Authorization", "Bearer "+token)
	return addHead
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

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
