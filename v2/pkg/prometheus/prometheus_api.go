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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func queryForPrometheusService(
	ctx context.Context,
	cc ClientCommandRunner,
	deployedNamespace string,
	req reconcile.Request,
) (*corev1.Service, error) {
	service := &corev1.Service{}

	name := types.NamespacedName{
		Name:      utils.PROMETHEUS_METERBASE_NAME,
		Namespace: deployedNamespace,
	}

	if result, _ := cc.Do(ctx, GetAction(name, service)); !result.Is(Continue) {
		return nil, errors.Wrap(result, "failed to get prometheus service")
	}

	log.Info("retrieved prometheus service")
	return service, nil
}

func getCertConfigMap(ctx context.Context, 
	cc ClientCommandRunner,
	deployedNamespace string, 
	req reconcile.Request) (*corev1.ConfigMap, error) {
	certConfigMap := &corev1.ConfigMap{}

	name := types.NamespacedName{
		Name:      utils.OPERATOR_CERTS_CA_BUNDLE_NAME,
		Namespace: deployedNamespace,
	}

	if result, _ := cc.Do(context.TODO(), GetAction(name, certConfigMap)); !result.Is(Continue) {
		return nil, errors.Wrap(result.GetError(), "Failed to retrieve operator-certs-ca-bundle.")
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
	request reconcile.Request)(*PrometheusAPI,error){
	service, err := queryForPrometheusService(context,cc,deployedNamespace,request)
	if err != nil {
		return nil, err
	}
	certConfigMap, err := getCertConfigMap(context,cc,deployedNamespace,request)
	if err != nil {
		return nil, err
	}
	saClient := NewServiceAccountClient(deployedNamespace, kubeInterface)
	authToken, err := saClient.NewServiceAccountToken(utils.OPERATOR_SERVICE_ACCOUNT, utils.PrometheusAudience, 3600, reqLogger)
	if err != nil {
		return nil, err
	}
	if certConfigMap != nil && authToken != "" && service != nil {
		cert, err := parseCertificateFromConfigMap(*certConfigMap)
		if err != nil {
			return nil, err
		}
		prometheusAPI, err := NewPromAPI(service, &cert, authToken)
		if err != nil {
			return nil, err
		}
		return prometheusAPI,nil
	}
	return nil,nil
}
