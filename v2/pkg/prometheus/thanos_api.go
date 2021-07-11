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

/* ThanosQuerierAPI is used when userWorkloadMonitoring is enabled, and a query needs to be made through the
Thanos Querier Service, which aggregates the Cluster & User metrics.
However, it does not provide Prometheus Targets information, so there are cases where we only query the UWM service
*/

func queryForThanosQuerierService(
	ctx context.Context,
	cc ClientCommandRunner,
) (*corev1.Service, error) {
	service := &corev1.Service{}

	name := types.NamespacedName{
		Name:      utils.OPENSHIFT_MONITORING_THANOS_QUERIER_SERVICE_NAME,
		Namespace: utils.OPENSHIFT_MONITORING_NAMESPACE,
	}

	if result, _ := cc.Do(ctx, GetAction(name, service)); !result.Is(Continue) {
		return nil, errors.Wrap(result, "failed to get thanos querier service")
	}

	log.Info("retrieved thanos querier service")
	return service, nil
}

func ProvideThanosQuerierAPI(
	context context.Context,
	cc ClientCommandRunner,
	kubeInterface kubernetes.Interface,
	deployedNamespace string,
	reqLogger logr.Logger) (*PrometheusAPI, error) {

	service, err := queryForThanosQuerierService(context, cc)
	if err != nil {
		return nil, err
	}

	certConfigMap, err := getCertConfigMap(context, cc, deployedNamespace, true)
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
		prometheusAPI, err := NewThanosAPI(service, &cert, authToken)
		if err != nil {
			return nil, err
		}
		return prometheusAPI, nil
	}
	return nil, nil
}
