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

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/prometheus/common/log"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

func queryForPrometheusService(
	ctx context.Context,
	cc ClientCommandRunner,
	deployedNamespace string,
	userWorkloadMonitoringEnabled bool,
) (*corev1.Service, *corev1.ServicePort, error) {
	service := &corev1.Service{}

	var name types.NamespacedName
	var portName string
	if userWorkloadMonitoringEnabled {
		name = types.NamespacedName{
			Name:      utils.OPENSHIFT_MONITORING_THANOS_QUERIER_SERVICE_NAME,
			Namespace: utils.OPENSHIFT_MONITORING_NAMESPACE,
		}
		portName = "http"
	} else {
		name = types.NamespacedName{
			Name:      utils.METERBASE_PROMETHEUS_SERVICE_NAME,
			Namespace: deployedNamespace,
		}
		portName = "rbac"
	}

	if result, _ := cc.Do(ctx, GetAction(name, service)); !result.Is(Continue) {
		return nil, nil, errors.Wrap(result, "failed to get prometheus service")
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
	cc ClientCommandRunner,
	deployedNamespace string,
	userWorkloadMonitoringEnabled bool) (*corev1.ConfigMap, error) {
	certConfigMap := &corev1.ConfigMap{}
	name := types.NamespacedName{
		Name:      utils.SERVING_CERTS_CA_BUNDLE_NAME,
		Namespace: deployedNamespace,
	}

	if result, _ := cc.Do(context.TODO(), GetAction(name, certConfigMap)); !result.Is(Continue) {
		return nil, errors.Wrap(result.GetError(), "Failed to retrieve serving-certs-ca-bundle.")
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
	cc ClientCommandRunner,
	kubeInterface kubernetes.Interface,
	deployedNamespace string,
	reqLogger logr.Logger,
	userWorkloadMonitoringEnabled bool) (*PrometheusAPI, error) {

	service, port, err := queryForPrometheusService(context, cc, deployedNamespace, userWorkloadMonitoringEnabled)
	if err != nil {
		return nil, err
	}

	certConfigMap, err := getCertConfigMap(context, cc, deployedNamespace, userWorkloadMonitoringEnabled)
	if err != nil {
		return nil, err
	}

	var saClient *ServiceAccountClient
	var authToken string
	saClient = NewServiceAccountClient(deployedNamespace, kubeInterface)
	authToken, err = saClient.NewServiceAccountToken(utils.OPERATOR_SERVICE_ACCOUNT, "", 3600, reqLogger)
	if err != nil {
		return nil, err
	}

	if certConfigMap != nil && authToken != "" && service != nil {
		cert, err := parseCertificateFromConfigMap(*certConfigMap)
		if err != nil {
			return nil, err
		}
		prometheusAPI, err := NewPromAPI(service, port, &cert, authToken)
		if err != nil {
			return nil, err
		}
		return prometheusAPI, nil
	}
	return nil, nil
}
