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
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/inject"
	utils "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/certificates"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ reconcile.Reconciler = &CertIssuerReconciler{}

// CertIssuerReconciler reconciles a CertIssuer object
type CertIssuerReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	certIssuer *utils.CertIssuer
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
	// ObjectMeta: metav1.ObjectMeta{
	// 	Annotations: map[string]string{
	// 		"service.beta.openshift.io/inject-cabundle": "true",
	// 	},
	// },
	// }

	selector := client.MatchingFields{"metadata.annotations.service.beta.openshift.io/inject-cabundle": "true"}
	err := r.Client.List(context.TODO(), configMapList, selector)
	if err != nil {
		fmt.Printf("\n\n\n\n\nERROR %+v\n\n\n\n", err)
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}
	fmt.Printf("\n\n\n\n\n%+v\n\n\n\n", configMapList)
	for _, cm := range configMapList.Items {
		if len(cm.Data["service-ca.crt"]) == 0 {
			err := r.InjectCACertIntoConfigMap(&cm)
			if err != nil {
				log.Error(err, "failed to inject CA certificate")
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *CertIssuerReconciler) Inject(injector *inject.Injector) inject.SetupWithManager {
	injector.SetCustomFields(r)
	return r
}

func (r *CertIssuerReconciler) InjectOperatorConfig(ci *utils.CertIssuer) error {
	r.certIssuer = ci
	return nil
}

// InjectCACertIntoConfigMap injects certificate data into
func (r *CertIssuerReconciler) InjectCACertIntoConfigMap(configmap *corev1.ConfigMap) error {
	cm := configmap
	cm.Data["service-ca.crt"] = string(r.certIssuer.CertificateAuthority.PublicKey)

	return r.Client.Patch(context.Background(), cm, nil)
}

// SetupWithManager sets up the controller with the Manager.
func (r *CertIssuerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&marketplaceredhatcomv1beta1.CertIssuer{}).
		Complete(r)
}
