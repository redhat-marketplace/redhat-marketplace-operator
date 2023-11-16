// Copyright 2021 IBM Corp.
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
	"context"
	"os"

	"emperror.dev/errors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func queryForPrometheusService(
	ctx context.Context,
	client client.Client,
	deployedNamespace string,
	apiType PrometheusAPIType,
) (*corev1.Service, *corev1.ServicePort, error) {
	service := &corev1.Service{}

	var name types.NamespacedName

	var portName string
	if apiType == UserWorkload {
		name = types.NamespacedName{
			Name:      utils.OPENSHIFT_MONITORING_THANOS_QUERIER_SERVICE_NAME,
			Namespace: utils.OPENSHIFT_MONITORING_NAMESPACE,
		}
		portName = "web"
	} else {
		name = types.NamespacedName{
			Name:      utils.METERBASE_PROMETHEUS_SERVICE_NAME,
			Namespace: deployedNamespace,
		}
		portName = "rbac"
	}

	if err := client.Get(ctx, name, service); err != nil {
		return nil, nil, errors.Wrap(err, "failed to get prometheus service")
	}

	log.Info("retrieved prometheus service")

	var port *corev1.ServicePort

	for i, portB := range service.Spec.Ports {
		if portB.Name == portName {
			port = &service.Spec.Ports[i]
		}
	}

	return service, port, nil
}

func getCertConfigMap(ctx context.Context,
	client client.Client,
	deployedNamespace string) (*corev1.ConfigMap, error) {
	certConfigMap := &corev1.ConfigMap{}
	name := types.NamespacedName{
		Name:      utils.SERVING_CERTS_CA_BUNDLE_NAME,
		Namespace: deployedNamespace,
	}
	_ = name

	if err := client.Get(ctx, name, certConfigMap); err != nil {
		return nil, errors.Wrap(err, "Failed to retrieve serving-certs-ca-bundle.")
	}

	log.Info("retrieved configmap")
	return certConfigMap, nil
}

func parseCertificateFromConfigMap(certConfigMap corev1.ConfigMap) (cert []byte, returnErr error) {
	log.Info("extracting cert from config map")

	out, ok := certConfigMap.Data["service-ca.crt"]

	if !ok {
		returnErr = errors.New("Error retrieving cert from config map")
		return nil, returnErr
	}

	cert = []byte(out)
	return cert, nil
}

func ProvidePrometheusAPI(
	context context.Context,
	client client.Client,
	deployedNamespace string,
	apiType PrometheusAPIType) (*PrometheusAPI, error) {
	service, port, err := queryForPrometheusService(context, client, deployedNamespace, apiType)
	if err != nil {
		return nil, err
	}

	certConfigMap, err := getCertConfigMap(context, client, deployedNamespace)
	if err != nil {
		return nil, err
	}

	authToken, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return nil, err
	}

	if certConfigMap != nil && service != nil {
		cert, err := parseCertificateFromConfigMap(*certConfigMap)
		if err != nil {
			return nil, err
		}
		prometheusAPI, err := NewPromAPI(service, port, &cert, string(authToken))
		if err != nil {
			return nil, err
		}
		return prometheusAPI, nil
	}
	return nil, nil
}
