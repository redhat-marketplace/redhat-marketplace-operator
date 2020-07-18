package reporter

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net/http"

	"emperror.dev/errors"
	"github.com/prometheus/client_golang/api"
)

type PrometheusSecureClientConfig struct {
	Address string

	ServerCertFile string
}

func NewSecureClient(config *PrometheusSecureClientConfig) (api.Client, error) {
	tlsConfig, err := generateCACertPool(config.ServerCertFile)

	if err != nil {
		return nil, errors.Wrap(err, "failed to get tlsConfig")
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	client, err := api.NewClient(api.Config{
		Address:      config.Address,
		RoundTripper: transport,
	})

	return client, err
}

func generateCACertPool(files ...string) (*tls.Config, error) {
	caCertPool := x509.NewCertPool()

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
