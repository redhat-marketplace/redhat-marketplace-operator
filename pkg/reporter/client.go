package reporter

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net/http"

	"github.com/prometheus/client_golang/api"
)

type PrometheusSecureClientConfig struct {
	Address string

	ServerCertFile string
}

func NewSecureClient(config *PrometheusSecureClientConfig) (api.Client, error) {
	caCert, err := ioutil.ReadFile(config.ServerCertFile)
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	transport := &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:      caCertPool,
			},
	}

	client, err := api.NewClient(api.Config{
		Address: config.Address,
		RoundTripper: transport,
	})

	return client, err
}
