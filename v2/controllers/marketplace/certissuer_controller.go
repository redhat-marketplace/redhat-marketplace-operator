/*
Copyright 2020 IBM Co..

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package marketplace

import (
	"context"
	"fmt"

	"github.com/cloudflare/cfssl/log"
	"github.com/go-logr/logr"
	marketplaceredhatcomv1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/inject"
	utils "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/certificates"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var _ reconcile.Reconciler = &CertIssuerReconciler{}

// CertIssuerReconciler reconciles a CertIssuer object
type CertIssuerReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	certIssuer *utils.CertIssuer
	cfg        *config.OperatorConfig
}

// +kubebuilder:rbac:groups=marketplace.redhat.com.redhat.com,resources=certissuers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=marketplace.redhat.com.redhat.com,resources=certissuers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=marketplace.redhat.com.redhat.com,resources=certissuers/finalizers,verbs=update

// Reconcile fills configmaps with tls certificates data
func (r *CertIssuerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	reqLogger := r.Log.WithValues("certissuer", req.NamespacedName)
	reqLogger.Info("Reconciling Certificates")

	// Fetch configmaps
	configMapList := &corev1.ConfigMapList{}
	opts := []client.ListOption{
		client.InNamespace(req.NamespacedName.Namespace),
	}
	err := r.Client.List(context.TODO(), configMapList, opts...)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}
	for _, cm := range configMapList.Items {
		if _, ok := cm.Annotations["service.beta.openshift.io/inject-cabundle"]; ok {
			if _, ok := cm.Data["service-ca.crt"]; ok {
				err := r.InjectCACertIntoConfigMap(&cm)
				if err != nil {
					log.Error(err, "failed to inject CA certificate")
				}
			}
		}
	}

	// Create missing TLS secrets
	tlsSecrets := []string{
		"prometheus-operator",
		"rhm-metric-state",
		"rhm-prometheus-meterbase",
	}

	err = r.CreateTLSSecrets(req.NamespacedName.Namespace, tlsSecrets...)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *CertIssuerReconciler) Inject(injector *inject.Injector) inject.SetupWithManager {
	injector.SetCustomFields(r)
	return r
}

func (r *CertIssuerReconciler) InjectCertIssuer(ci *utils.CertIssuer) error {
	r.certIssuer = ci
	return nil
}

func (r *CertIssuerReconciler) InjectOperatorConfig(oc *config.OperatorConfig) error {
	r.cfg = oc
	return nil
}

// InjectCACertIntoConfigMap injects certificate data into
func (r *CertIssuerReconciler) InjectCACertIntoConfigMap(configmap *corev1.ConfigMap) error {
	cm := configmap
	patch := client.MergeFrom(cm.DeepCopy())
	cm.Data["service-ca.crt"] = string(r.certIssuer.CAPublicKey())
	return r.Client.Patch(context.Background(), cm, patch)
}

// CreateTLSSecrets creates tls secrets signed by recociler's CA
func (r *CertIssuerReconciler) CreateTLSSecrets(namespace string, secretNames ...string) error {
	cert, key, err := r.certIssuer.CreateCertFromCA(types.NamespacedName{
		Name:      secretNames[0],
		Namespace: namespace,
	})
	if err != nil {
		return err
	}
	secretName := fmt.Sprintf("%s-tls", secretNames[0])
	secretMeta := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      secretName,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": cert,
			"tls.key": key,
		},
	}

	err = r.Client.Create(context.Background(), secretMeta)
	if err != nil {
		r.Log.Error(err, "cannot create tls secret")
	}

	r.Log.Info("created tls secret", "secretName", secretName)

	if len(secretNames) == 1 {
		return nil
	}

	return r.CreateTLSSecrets(namespace, secretNames[1:]...)
}

// SetupWithManager sets up the controller with the Manager.
func (r *CertIssuerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.cfg.Infrastructure.HasOpenshift() {
		return nil
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&marketplaceredhatcomv1beta1.CertIssuer{}).
		Watches(
			&source.Kind{Type: &corev1.ConfigMap{}},
			&handler.EnqueueRequestForOwner{
				IsController: true,
				OwnerType:    &marketplaceredhatcomv1beta1.CertIssuer{}}).
		Complete(r)
}
