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

package reporter

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"

	"emperror.dev/errors"
	"github.com/prometheus/client_golang/api"
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

func NewSecureClient(config *PrometheusSecureClientConfig) (api.Client, error) {
	tlsConfig, err := generateCACertPool(config.ServerCertFile)

	if err != nil {
		return nil, errors.Wrap(err, "failed to get tlsConfig")
	}

	var transport http.RoundTripper

	transport = &http.Transport{
		TLSClientConfig: tlsConfig,
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

func NewSecureClientFromConfigMap(config *PrometheusSecureClientConfig) (api.Client, error) {
	tlsConfig, err := generateCACertPoolFromConfigMap(*config.CaCert)
	fmt.Println("TLSCONFIG",tlsConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tlsConfig")
	}

	var transport http.RoundTripper

	transport = &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	if config.UserAuth != nil {
		transport = WithBasicAuth(transport, config.UserAuth.Username, config.UserAuth.Password)
	}

	if config.Token != "" {
		transport = WithBearerAuth(transport, config.Token)
	}

	fmt.Println("TRANSPORT :",transport)
	client, err := api.NewClient(api.Config{
		Address:      config.Address,
		RoundTripper: transport,
	})

	fmt.Println("CLIENT FROM NewSecureClientFromConfigMap",client)
	return client, err
}

func generateCACertPoolFromConfigMap(caCert []byte) (*tls.Config, error) {
	caCertPool, err := x509.SystemCertPool()

	if err != nil {
		return nil, errors.Wrap(err, "failed to get system cert pool")
	}

	// for _, file := range files {
	// 	caCert, err := ioutil.ReadFile(file)
	// 	if err != nil {
	// 		return nil, errors.Wrap(err, "failed to load cert file")
	// 	}
	// 	caCertPool.AppendCertsFromPEM(caCert)
	// }

	caCertPool.AppendCertsFromPEM(caCert)

	return &tls.Config{
		RootCAs: caCertPool,
	}, nil
}

func generateCACertPool(files ...string) (*tls.Config, error) {
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
