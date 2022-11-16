// Copyright 2020 IBM Corp.
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

package marketplace

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/builder"

	"github.com/go-logr/logr"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	utils "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

//var log = logf.Log.WithName("controller_olm_clusterserviceversion_watcher")

const (
	watchTag         string = "razee/watch-resource"
	olmCopiedFromTag string = "olm.copiedFrom"
	olmNamespace     string = "olm.operatorNamespace"
	ignoreTag        string = "marketplace.redhat.com/ignore"
	ignoreTagValue   string = "2"
	meterDefStatus   string = "marketplace.redhat.com/meterDefinitionStatus"
	meterDefError    string = "marketplace.redhat.com/meterDefinitionError"
)

// blank assignment to verify that ReconcileClusterServiceVersion implements reconcile.Reconciler
var _ reconcile.Reconciler = &ClusterServiceVersionReconciler{}

// ClusterServiceVersionReconciler reconciles a ClusterServiceVersion object
type ClusterServiceVersionReconciler struct {
	// This Client, initialized using mgr.Client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	Client client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

// +kubebuilder:rbac:groups="operators.coreos.com",resources=clusterserviceversions;subscriptions,verbs=get;list;watch
// +kubebuilder:rbac:groups="operators.coreos.com",resources=clusterserviceversions,verbs=update;patch
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=meterdefinitions;meterdefinitions/status,verbs=get;list;watch

// Reconcile reads that state of the cluster for a ClusterServiceVersion object and makes changes based on the state read
// and what is in the ClusterServiceVersion.Spec
func (r *ClusterServiceVersionReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Name", request.Name, "Request.Namespace", request.Namespace)
	reqLogger.Info("Reconciling ClusterServiceVersion")
	// Fetch the ClusterServiceVersion instance
	CSV := &olmv1alpha1.ClusterServiceVersion{}
	err := r.Client.Get(context.TODO(), request.NamespacedName, CSV)

	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Error(err, "clusterserviceversion does not exist")
			return reconcile.Result{}, nil
		}
		reqLogger.Error(err, "Failed to get clusterserviceversion")
		return reconcile.Result{}, err
	}

	annotations := CSV.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	sub := &olmv1alpha1.SubscriptionList{}

	if err := r.Client.List(context.TODO(), sub, client.InNamespace(request.NamespacedName.Namespace)); err != nil {
		return reconcile.Result{}, err
	}

	if len(sub.Items) > 0 {
		reqLogger.V(4).Info("found Subscription in namespaces", "count", len(sub.Items))
		// add razee watch label to CSV if subscription has rhm/operator label
		for _, s := range sub.Items {
			if value, ok := s.GetLabels()[utils.OperatorTag]; ok {
				if value == "true" {
					if len(s.Status.InstalledCSV) == 0 {
						reqLogger.Info("Requeue clusterserviceversion to wait for subscription getting installedCSV updated")
						return reconcile.Result{RequeueAfter: time.Second * 5}, nil
					}

					if s.Status.InstalledCSV == request.NamespacedName.Name {
						reqLogger.Info("found Subscription with installed CSV")

						if v, ok := CSV.GetLabels()[watchTag]; !ok || v != "lite" {
							if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
								if err := r.Client.Get(context.TODO(),
									types.NamespacedName{
										Name:      CSV.GetName(),
										Namespace: CSV.GetNamespace(),
									},
									CSV); err != nil {
									return err
								}

								labels := CSV.GetLabels()

								if labels == nil {
									labels = make(map[string]string)
								}

								labels[watchTag] = "lite"
								CSV.SetLabels(labels)

								return r.Client.Update(context.TODO(), CSV)
							}); err != nil {
								reqLogger.Error(err, "Failed to patch clusterserviceversion with razee/watch-resource: lite label")
								return reconcile.Result{}, err
							}
							reqLogger.Info("Patched clusterserviceversion with razee/watch-resource: lite label")
						} else {
							reqLogger.Info("No patch needed on clusterserviceversion resource")
						}
					}
				}
			}
		}
	} else {
		reqLogger.Info("Did not find Subscription in namespace")
	}

	reqLogger.Info("reconciliation complete")
	return reconcile.Result{}, nil
}

// The head CSV (not a copy)
func csvFilter(metaNew metav1.Object) bool {
	ann := metaNew.GetAnnotations()
	labels := metaNew.GetLabels()

	_, hasCopiedFrom := labels[olmCopiedFromTag]
	_, hasOlmNamespace := ann[olmNamespace]

	if hasOlmNamespace && !hasCopiedFrom {
		return true
	}

	return false
}

var clusterServiceVersionPredictates predicate.Funcs = predicate.Funcs{
	UpdateFunc: func(evt event.UpdateEvent) bool {
		return csvFilter(evt.ObjectNew)
	},
	DeleteFunc: func(evt event.DeleteEvent) bool {
		return false
	},
	CreateFunc: func(evt event.CreateEvent) bool {
		return csvFilter(evt.Object)
	},
	GenericFunc: func(evt event.GenericEvent) bool {
		return false
	},
}

func (r *ClusterServiceVersionReconciler) SetupWithManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&olmv1alpha1.ClusterServiceVersion{}, builder.WithPredicates(clusterServiceVersionPredictates)).
		Watches(
			&source.Kind{Type: &marketplacev1beta1.MeterDefinition{}}, &handler.EnqueueRequestForOwner{
				IsController: false,
				OwnerType:    &olmv1alpha1.ClusterServiceVersion{},
			}).
		Complete(r)
}
