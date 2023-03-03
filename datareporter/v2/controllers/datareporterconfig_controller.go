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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"
	v1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/api/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/events"
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
// +kubebuilder:rbac:groups="",namespace=system,resources=secrets,verbs=get;list;watch

func (r *DataReporterConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)

	dataReporterConfig := &v1alpha1.DataReporterConfig{}
	if err := r.Client.Get(ctx, req.NamespacedName, dataReporterConfig); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("datareporterconfig resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		reqLogger.Error(err, "Failed to get datareporterconfig")
		return ctrl.Result{}, err
	}
	reqLogger.Info("datareporterconfig found")

	// decoded secret/metadata pairs
	apiKeys := []events.ApiKey{}

	for _, apiKey := range dataReporterConfig.Spec.ApiKeys {
		// only handle namespace local secrets
		if apiKey.SecretReference.Namespace == dataReporterConfig.GetNamespace() || apiKey.SecretReference.Namespace != "" {
			secret := &corev1.Secret{}
			// If we don't find the secret, log an error and continue
			if err := r.Client.Get(ctx, types.NamespacedName{Name: apiKey.SecretReference.Name}, secret); errors.IsNotFound(err) {
				reqLogger.Error(err, fmt.Sprintf("secret/%s referenced in datareporterconfig not found", apiKey.SecretReference.Name))
			} else if err != nil {
				reqLogger.Error(err, "Failed to get secret")
			} else {
				// Verify the secret has an X-API-KEY, and append to apiKeys
				reqLogger.Info(fmt.Sprintf("secret/%s found", secret.Name))
				keyBytes, ok := secret.Data["X-API-KEY"]
				key := events.Key(string(keyBytes))
				if ok {
					apiKeys = append(apiKeys, events.ApiKey{Key: key, Metadata: apiKey.Metadata})
				} else {
					reqLogger.Error(err, fmt.Sprintf("No X-API-KEY in secret/%s referenced in datareporterconfig", apiKey.SecretReference.Name))
				}
			}

		}
	}
	// Set the X-API-KEY secret and the metadata pairs in the event Config
	r.Config.ApiKeys.SetApiKeys(apiKeys)

	reqLogger.Info("reconcile complete")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
// reconcile only datareporterconfig, and when Secrets change in the namespace
func (r *DataReporterConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {

	predicate := predicate.Funcs{
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
		For(&v1alpha1.DataReporterConfig{}).
		Watches(
			&source.Kind{Type: &corev1.Secret{}},
			handler.EnqueueRequestsFromMapFunc(mapFn)).
		Watches(
			&source.Kind{Type: &v1alpha1.DataReporterConfig{}},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(predicate)).
		Complete(r)
}
