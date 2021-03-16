/*
Copyright 2021 IBM Co..

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
	"crypto/x509"
	"fmt"
	"strconv"
	"time"

	"github.com/cloudflare/cfssl/helpers"
	"github.com/cloudflare/cfssl/log"
	"github.com/go-logr/logr"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	utils "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/certificates"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/predicates"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var _ reconcile.Reconciler = &CertIssuerReconciler{}

const InjectCAAnnotation = "service.beta.openshift.io/inject-cabundle"

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
	cm := &corev1.ConfigMap{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, cm)
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

	if _, ok := cm.Annotations[InjectCAAnnotation]; ok {
		if _, ok := cm.Data["service-ca.crt"]; ok {
			err := r.InjectCACertIntoConfigMap(cm)
			if err != nil {
				log.Error(err, "failed to inject CA certificate")
				return ctrl.Result{}, err
			}
		} else {
			reqLogger.V(1).Info("service-ca.crt field was not found in a configmap labeled with inject-cabundle")
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

	return ctrl.Result{RequeueAfter: time.Hour * 24}, nil
}

func (r *CertIssuerReconciler) Inject(injector mktypes.Injectable) mktypes.SetupWithManager {
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
	if configmap.Data["service-ca.crt"] == string(r.certIssuer.CAPublicKey()) {
		return nil
	}
	cm := configmap
	patch := client.MergeFrom(cm.DeepCopy())
	cm.Data["service-ca.crt"] = string(r.certIssuer.CAPublicKey())
	return r.Client.Patch(context.Background(), cm, patch)
}

func (r *CertIssuerReconciler) CreateCASecret(namespace string) error {
	sec := &corev1.Secret{}
	err := r.Client.Get(context.Background(), types.NamespacedName{
		Name:      "rhmp-ca-tls",
		Namespace: namespace,
	}, sec)

	parsedCACert, err := x509.ParseCertificate(r.certIssuer.CAPublicKey())
	if err != nil {
		return err
	}

	ownerService := &corev1.Service{}
	err := r.Client.Get(context.Background(), types.NamespacedName{
		Name:      "redhat-marketplace-controller-manager-service",
		Namespace: namespace,
	}, ownerService)
	if err != nil {
		return err
	}

	expiresOn := helpers.ExpiryTime([]*x509.Certificate{parsedCACert})
	secretMeta := &corev1.Secret{
		Annotations: map[string]string{
			"certificate/ExpiresOn": expiresOn.Unix(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "rhmp-ca-tls",
			OwnerReferences: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion:         ownerService.APIVersion,
					Kind:               ownerService.Kind,
					Name:               ownerService.Name,
					UID:                ownerService.UID,
					BlockOwnerDeletion: pointer.BoolPtr(false),
				},
			},
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": r.certIssuer.CAPublicKey(),
			"tls.key": r.certIssuer.CAPrivateKey(),
		},
	}

	return r.Client.Create(context.Background(), secretMeta)
}

// CreateTLSSecrets creates tls secrets signed by recociler's CA
func (r *CertIssuerReconciler) CreateTLSSecrets(namespace string, secretNames ...string) error {
	sec := &corev1.Secret{}
	err := r.Client.Get(context.Background(), types.NamespacedName{
		Name:      secretNames[0],
		Namespace: namespace,
	}, sec)

	secretDoesntExists := err != nil && errors.IsNotFound(err)
	if !secretDoesntExists {
		if _, ok := sec.Annotations["service-ca.crt"]; ok {
			parsedUnixTime, err := strconv.ParseInt(sec.Annotations["certificate/ExpiresOn"], 10, 64)
			if err == nil {
				return err
			}
			expiresIn := time.Unix(parsedUnixTime).Sub(time.Now()).Hours()
			if expiresIn < 24 {
				// Update
			}
		}
		return r.CreateTLSSecrets(namespace, secretNames[1:]...)
	}

	cert, key, err := r.certIssuer.CreateCertFromCA(types.NamespacedName{
		Name:      secretNames[0],
		Namespace: namespace,
	})
	if err != nil {
		return err
	}
	secretName := fmt.Sprintf("%s-tls", secretNames[0])
	parsedCert, err := x509.ParseCertificate(cert)
	if err != nil {
		return err
	}
	expiresOn := helpers.ExpiryTime([]*x509.Certificate{parsedCert})
	secretMeta := &corev1.Secret{
		Annotations: map[string]string{
			"certificate/ExpiresOn": expiresOn.Unix(),
		},
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
		return err
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

	namespacePredicate := predicates.NamespacePredicate(r.cfg.DeployedNamespace)
	configmapPreds := []predicate.Predicate{
		predicate.Funcs{
			UpdateFunc: func(evt event.UpdateEvent) bool {
				_, ok := evt.MetaOld.GetAnnotations()[InjectCAAnnotation]
				return ok
			},
			CreateFunc: func(evt event.CreateEvent) bool {
				_, ok := evt.Meta.GetAnnotations()[InjectCAAnnotation]
				return ok
			},
			DeleteFunc: func(evt event.DeleteEvent) bool {
				_, ok := evt.Meta.GetAnnotations()[InjectCAAnnotation]
				return ok
			},
			GenericFunc: func(evt event.GenericEvent) bool {
				_, ok := evt.Meta.GetAnnotations()[InjectCAAnnotation]
				return ok
			},
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(namespacePredicate).
		For(&corev1.ConfigMap{}, builder.WithPredicates(configmapPreds...)).
		Watches(&source.Kind{Type: &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind: "Secret",
			},
		}}, &handler.EnqueueRequestForOwner{
			IsController: false,
			OwnerType:    &marketplacev1alpha1.MarketplaceConfig{},
		}).
		Complete(r)
}
