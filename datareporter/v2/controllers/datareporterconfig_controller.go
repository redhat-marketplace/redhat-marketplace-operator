/*
Copyright 2023 IBM Co..

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

package controllers

import (
	"context"
	"crypto/rand"
	"fmt"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/gotidy/ptr"
	"github.com/imdario/mergo"
	routev1 "github.com/openshift/api/route/v1"
	v1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/api/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/events"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// DataReporterConfigReconciler reconciles a DataReporterConfig object
type DataReporterConfigReconciler struct {
	Client client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
	Config *events.Config
}

// data-service
//+kubebuilder:rbac:urls=/dataservice.v1.fileserver.FileServer/*,verbs=create
// kube-rbac-proxy
//+kubebuilder:rbac:groups=authentication.k8s.io,resources=tokenreviews,verbs=create
//+kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews,verbs=create

// controller CRDs
//+kubebuilder:rbac:groups=marketplace.redhat.com,resources=datareporterconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=marketplace.redhat.com,resources=datareporterconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=marketplace.redhat.com,resources=datareporterconfigs/finalizers,verbs=update
//+kubebuilder:rbac:groups=marketplace.redhat.com,resources=marketplaceconfigs,verbs=get;list;watch
//+kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",namespace=system,resources=secrets,verbs=get;list;watch;create

func (r *DataReporterConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)

	dataReporterConfig := &v1alpha1.DataReporterConfig{}
	if err := r.Client.Get(ctx, req.NamespacedName, dataReporterConfig); err != nil {
		if k8serrors.IsNotFound(err) {
			reqLogger.Info("datareporterconfig resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		reqLogger.Error(err, "Failed to get datareporterconfig")
		return ctrl.Result{}, err
	}
	reqLogger.Info("datareporterconfig found")

	// decoded secret/metadata pairs
	apiKeys := []events.ApiKey{}

	// prerequisite marketplaceconfig, otherwise deny events by denying all api keys
	marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: utils.MARKETPLACECONFIG_NAME, Namespace: req.Namespace}, marketplaceConfig); err != nil {
		if k8serrors.IsNotFound(err) {
			reqLogger.Info("marketplaceconfig resource not found. IBM Metrics Operator prerequisite required.")
			r.Config.ApiKeys.SetApiKeys(apiKeys)
			return ctrl.Result{}, nil
		}
		reqLogger.Error(err, "Failed to get marketplaceconfig")
		return ctrl.Result{}, err
	}

	// must accept license, otherwise deny events by denying all api keys
	if ptr.ToBool(marketplaceConfig.Spec.License.Accept) != true {
		r.Config.ApiKeys.SetApiKeys(apiKeys)
		reqLogger.Info("license must be accepted in marketplaceconfig to receive events.")
		return ctrl.Result{}, nil
	}

	for _, apiKey := range dataReporterConfig.Spec.ApiKeys {
		// only handle namespace local secrets
		if apiKey.SecretReference.Namespace == dataReporterConfig.GetNamespace() || apiKey.SecretReference.Namespace == "" {
			secret := &corev1.Secret{}
			// If we don't find the secret, generate a new secret with an X-API-KEY
			if err := r.Client.Get(ctx, types.NamespacedName{Name: apiKey.SecretReference.Name, Namespace: dataReporterConfig.GetNamespace()}, secret); k8serrors.IsNotFound(err) {
				reqLogger.Info("secret referenced in datareporterconfig not found, generating new secret with random X-API-KEY", "secret", apiKey.SecretReference.Name)

				secret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      apiKey.SecretReference.Name,
						Namespace: dataReporterConfig.GetNamespace(),
					},
					Data: map[string][]byte{
						"X-API-KEY": []byte(generateKey()),
					},
				}

				if err := r.Client.Create(ctx, secret); err != nil {
					return ctrl.Result{}, err
				}

			} else if err != nil {
				return ctrl.Result{}, err
			}

			// Verify the secret has an X-API-KEY, and append to apiKeys
			reqLogger.Info("secret found", "secret", secret.Name)
			keyBytes, ok := secret.Data["X-API-KEY"]
			key := events.Key(string(keyBytes))
			if ok {
				apiKeys = append(apiKeys, events.ApiKey{Key: key, Metadata: apiKey.Metadata})
			} else {
				reqLogger.Error(errors.New("no X-API-KEY found in secret"), secret.Name)
			}
		}
	}
	// Set the X-API-KEY secret and the metadata pairs in the event Config
	r.Config.ApiKeys.SetApiKeys(apiKeys)

	// Enforce the Service configuration for ibm-data-reporter-operator-controller-manager-metrics-service
	// TODO

	// Configure the Route
	route := routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ibm-data-reporter",
			Namespace: dataReporterConfig.Namespace,
		},
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: "ibm-data-reporter-operator-controller-manager-metrics-service",
			},
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromInt(8443),
			},
			TLS: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationReencrypt,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			},
		},
	}

	// Create the Route
	newRoute := route
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		_, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, &newRoute, func() error {
			controllerutil.SetControllerReference(dataReporterConfig, &newRoute, r.Scheme)
			return mergo.Merge(&newRoute, &route, mergo.WithOverride)
		})
		return err
	}); err != nil {
		return ctrl.Result{}, err
	}

	reqLogger.Info("reconcile complete")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
// reconcile only datareporterconfig, and when Secrets change in the namespace
func (r *DataReporterConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {

	drcPred := predicate.Funcs{
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.ObjectNew.GetName() == "datareporterconfig"
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return e.Object.GetName() == "datareporterconfig"
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return e.Object.GetName() == "datareporterconfig"
		},
	}

	mapFn := func(a client.Object) []reconcile.Request {
		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      "datareporterconfig",
					Namespace: a.GetNamespace(),
				},
			},
		}
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.DataReporterConfig{},
			builder.WithPredicates(drcPred)).
		Watches(
			&source.Kind{Type: &corev1.Secret{}},
			handler.EnqueueRequestsFromMapFunc(mapFn)).
		Watches(
			&source.Kind{Type: &routev1.Route{}},
			&handler.EnqueueRequestForOwner{
				IsController: true,
				OwnerType:    &v1alpha1.DataReporterConfig{}}).
		Complete(r)
}

func generateKey() string {
	b := make([]byte, 32)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}
