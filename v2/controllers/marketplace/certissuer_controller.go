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
	"encoding/pem"
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
	v1 "k8s.io/api/core/v1"
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
	err := r.Client.Get(context.Background(), req.NamespacedName, cm)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue

			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}
	err = r.UpdateCAResources(cm)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.InjectCACertIntoConfigMap(cm)
	if err != nil {
		log.Error(err, "failed to inject CA certificate")
		return ctrl.Result{}, err
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

	err = r.CreateCASecret(req.NamespacedName.Namespace)
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
	if _, ok := configmap.Annotations[InjectCAAnnotation]; !ok {
		return fmt.Errorf("configmap is missing certificate inject annotation")
	}
	if _, ok := configmap.Data["service-ca.crt"]; !ok {
		return fmt.Errorf("configmap data is missing service-ca.crt field")
	}

	if configmap.Data["service-ca.crt"] == string(r.certIssuer.CAPublicKey()) {
		return nil
	}

	patch := client.MergeFrom(configmap.DeepCopy())
	cm := *configmap
	cm.Data["service-ca.crt"] = string(r.certIssuer.CAPublicKey())
	return r.Client.Patch(context.Background(), &cm, patch)
}

func (r *CertIssuerReconciler) CreateCASecret(namespace string) error {
	sec := &corev1.Secret{}
	err := r.Client.Get(context.Background(), types.NamespacedName{
		Name:      "rhmp-ca-tls",
		Namespace: namespace,
	}, sec)
	if err != nil && !errors.IsNotFound(err) {

		return err
	}

	block, _ := pem.Decode(r.certIssuer.CAPublicKey())
	parsedCACert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}

	ownerService := &corev1.Service{}
	err = r.Client.Get(context.Background(), types.NamespacedName{
		Name:      "redhat-marketplace-controller-manager-service",
		Namespace: namespace,
	}, ownerService)
	if err != nil {
		return err
	}

	expiresOn := helpers.ExpiryTime([]*x509.Certificate{parsedCACert})
	secretMeta := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"certificate/ExpiresOn": fmt.Sprintf("%v", expiresOn.Unix()),
			},
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

	err = r.Client.Create(context.Background(), secretMeta)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			return r.RotateCASecretIfExpiresSoon(secretMeta)
		}

		return err
	}

	return nil
}

func (r *CertIssuerReconciler) UpdateCAResources(configmap *v1.ConfigMap) error {
	expiryTime, err := r.certIssuer.CAExpiresInHours()
	if err != nil {
		return err
	}

	if expiryTime > 24 {
		return nil
	}

	namespace := configmap.GetNamespace()
	err = r.certIssuer.RotateCA()
	if err != nil {
		return err
	}

	err = r.InjectCACertIntoConfigMap(configmap)
	if err != nil {
		return err
	}

	// Update CA secret
	sec := &corev1.Secret{}
	err = r.Client.Get(context.Background(), types.NamespacedName{
		Name:      "rhmp-ca-tls",
		Namespace: namespace,
	}, sec)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.CreateCASecret(namespace)
		}
		return err
	}

	cm := *configmap
	patch := client.MergeFrom(cm.DeepCopy())
	cm.Data["service-ca.crt"] = string(r.certIssuer.CAPublicKey())
	return r.Client.Patch(context.Background(), &cm, patch)
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
	block, _ := pem.Decode(cert)
	parsedCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}

	expiresOn := helpers.ExpiryTime([]*x509.Certificate{parsedCert})
	secretMeta := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"certificate/ExpiresOn": fmt.Sprintf("%v", expiresOn.Unix()),
			},
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
		if errors.IsAlreadyExists(err) {
			r.Log.Info("secret already exist", "secretName", secretName)
			rotateErr := r.RotateSingleSecretIfExpiresSoon(secretMeta, secretNames[0])
			if rotateErr != nil {
				r.Log.Error(err, "unable to rotate cert", secretName)
			}
			servicesList := &corev1.SecretList{}
			x := []client.ListOption{client.InNamespace(namespace)}
			err = r.Client.List(context.Background(), servicesList, x...)
			if err != nil {
				return err
			}

		} else {
			return err
		}
	} else {
		r.Log.Info("created tls secret", "secretName", secretName)
	}

	if len(secretNames) == 1 {
		return nil
	}

	return r.CreateTLSSecrets(namespace, secretNames[1:]...)
}

func (r *CertIssuerReconciler) RotateCASecretIfExpiresSoon(sec *corev1.Secret) error {
	_, ok := sec.Annotations["certificate/ExpiresOn"]
	if !ok {
		return fmt.Errorf("Cannot rotate certificate - annotation certificate/ExpiresOn not found")
	}

	parsedUnixTime, err := strconv.ParseInt(sec.Annotations["certificate/ExpiresOn"], 10, 64)
	if err != nil {
		return err
	}

	expiresIn := time.Until(time.Unix(parsedUnixTime, 0).UTC()).Hours()
	if expiresIn > 24 {
		return nil
	}

	err = r.Client.Delete(context.Background(), sec)
	if err != nil {
		return err
	}

	r.Log.Info("rotating secret", "secret name", sec.Name)
	return r.CreateCASecret(sec.Namespace)
}

func (r *CertIssuerReconciler) RotateSingleSecretIfExpiresSoon(secretMeta *corev1.Secret, name string) error {
	sec := &corev1.Secret{}
	err := r.Client.Get(context.Background(), types.NamespacedName{
		Name:      secretMeta.Name,
		Namespace: secretMeta.Namespace,
	}, sec)
	if err != nil {
		return err
	}

	_, ok := sec.Annotations["certificate/ExpiresOn"]
	if !ok {
		return fmt.Errorf("Cannot rotate certificate - annotation certificate/ExpiresOn not found")
	}

	parsedUnixTime, err := strconv.ParseInt(sec.Annotations["certificate/ExpiresOn"], 10, 64)
	if err != nil {
		return err
	}

	expiresIn := time.Until(time.Unix(parsedUnixTime, 0).UTC()).Hours()
	if expiresIn > 24 {
		return nil
	}

	cert, key, err := r.certIssuer.CreateCertFromCA(types.NamespacedName{
		Name:      name,
		Namespace: secretMeta.Namespace,
	})
	if err != nil {
		return err
	}

	sec.Data = map[string][]byte{
		"tls.crt": cert,
		"tls.key": key,
	}
	sec.Annotations = map[string]string{
		"certificate/ExpiresOn": fmt.Sprintf("%v", time.Now().AddDate(0, 0, 30).Unix()),
	}

	r.Log.Info("rotating secret", "secret name", sec.Name)
	return r.Client.Update(context.Background(), sec)
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
