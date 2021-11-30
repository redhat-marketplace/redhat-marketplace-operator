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
	"fmt"

	"emperror.dev/errors"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func NewThanosAPI(
	promService *corev1.Service,
	caCert *[]byte,
	token string,
) (*PrometheusAPI, error) {
	promAPI, err := provideThanosAPI(promService, caCert, token)
	if err != nil {
		return nil, err
	}
	prometheusAPI := &PrometheusAPI{promAPI}
	return prometheusAPI, nil
}

func provideThanosAPI(
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

	targetPort := intstr.FromString("web")

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
