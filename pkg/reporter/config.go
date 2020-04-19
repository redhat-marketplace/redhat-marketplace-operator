package reporter

import (
	"github.com/prometheus/client_golang/api"
	"github.ibm.com/symposium/redhat-marketplace-operator/pkg/apis"
	marketplacev1alpha1 "github.ibm.com/symposium/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.ibm.com/symposium/redhat-marketplace-operator/pkg/managers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	k8sconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

type ReporterName types.NamespacedName

type MarketplaceReporterConfig struct {
	Name       types.NamespacedName
	ObjectMeta metav1.ObjectMeta
	Config     marketplacev1alpha1.MeterReportSpec
	apiConfig  api.Config
	k8sConfig  *rest.Config
	clientset  *kubernetes.Clientset
}

func NewMarketplaceReporterConfig(
	reportName ReporterName,
	schemes []*managers.SchemeDefinition,
) (*MarketplaceReporterConfig, error) {
	// Get a config to talk to the apiserver
	cfg, err := k8sconfig.GetConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Error(err, "failed to get clientset")
		return nil, err
	}

	scheme := k8sscheme.Scheme

	if err := apis.AddToScheme(scheme); err != nil {
		log.Error(err, "failed to add scheme")
		return nil, err
	}

	for _, apiScheme := range schemes {
		log.Info("adding scheme", "scheme", apiScheme.Name)
		if err := apiScheme.AddToScheme(scheme); err != nil {
			log.Error(err, "failed to add scheme")
			return nil, err
		}
	}

	return &MarketplaceReporterConfig{
		Name: (types.NamespacedName)(reportName),
		k8sConfig: cfg,
		clientset: clientset,
	}, nil
}
