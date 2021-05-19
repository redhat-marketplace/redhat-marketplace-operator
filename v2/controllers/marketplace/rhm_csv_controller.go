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
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
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
	_watchTag         string = "razee/watch-resource"
	_olmCopiedFromTag string = "olm.copiedFrom"
	_ignoreTag        string = "marketplace.redhat.com/ignore"
	_ignoreTagValue   string = "2"
	_meterDefStatus   string = "marketplace.redhat.com/meterDefinitionStatus"
	_meterDefError    string = "marketplace.redhat.com/meterDefinitionError"
)

// blank assignment to verify that ReconcileClusterServiceVersion implements reconcile.Reconciler
var _ reconcile.Reconciler = &RhmCSVReconciler{}

// RhmCSVReconciler reconciles a ClusterServiceVersion object
type RhmCSVReconciler struct {
	// This Client, initialized using mgr.Client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	Client client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
	cfg            *config.OperatorConfig
}

// +kubebuilder:rbac:groups="operators.coreos.com",resources=clusterserviceversions;subscriptions,verbs=get;list;watch
// +kubebuilder:rbac:groups="operators.coreos.com",resources=clusterserviceversions,verbs=update;patch
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=meterdefinitions;meterdefinitions/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=meterdefinitions;meterdefinitions/status,verbs=get;list;watch;create;update;patch;delete

// Reconcile reads that state of the cluster for a ClusterServiceVersion object and makes changes based on the state read
// and what is in the ClusterServiceVersion.Spec
func (r *RhmCSVReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
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

	hasMarketplaceSub := false
	if len(sub.Items) > 0 {
		reqLogger.V(4).Info("found Subscription in namespaces", "count", len(sub.Items))
		// add razee watch label to CSV if subscription has rhm/operator label
		for _, s := range sub.Items {
			if value, ok := s.GetLabels()[operatorTag]; ok {
				if value == "true" {
					if len(s.Status.InstalledCSV) == 0 {
						reqLogger.Info("Requeue clusterserviceversion to wait for subscription getting installedCSV updated")
						return reconcile.Result{RequeueAfter: time.Second * 5}, nil
					}

					if s.Status.InstalledCSV == request.NamespacedName.Name {
						reqLogger.Info("found Subscription with installed CSV")
						hasMarketplaceSub = true

						if v, ok := CSV.GetLabels()[watchTag]; !ok || v != "lite" {
							err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
								err := r.Client.Get(context.TODO(),
									types.NamespacedName{
										Name:      CSV.GetName(),
										Namespace: CSV.GetNamespace(),
									},
									CSV)

								if err != nil {
									return err
								}

								labels := CSV.GetLabels()

								if labels == nil {
									labels = make(map[string]string)
								}

								labels[watchTag] = "lite"
								CSV.SetLabels(labels)

								return r.Client.Update(context.TODO(), CSV)
							})

							if err != nil {
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
		reqLogger.Info("Did not find Subscription in namespaces")
	}

	if !hasMarketplaceSub {
		reqLogger.Info("Does not have marketplace sub, ignoring CSV for future")
		if v, ok := CSV.GetAnnotations()[ignoreTag]; !ok || v != ignoreTagValue {
			retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := r.Client.Get(context.TODO(),
					types.NamespacedName{
						Name:      CSV.GetName(),
						Namespace: CSV.GetNamespace(),
					},
					CSV)

				if err != nil {
					return err
				}

				annotations := CSV.GetAnnotations()

				if annotations == nil {
					annotations = make(map[string]string)
				}

				annotations[ignoreTag] = ignoreTagValue
				CSV.SetAnnotations(annotations)

				return r.Client.Update(context.TODO(), CSV)
			})

			if retryErr != nil {
				reqLogger.Error(retryErr, "Failed to patch clusterserviceversion ignore tag")
				return reconcile.Result{Requeue: true}, retryErr
			}
			reqLogger.V(4).Info("Patched clusterserviceversion with ignore tag")
		} else {
			reqLogger.V(4).Info("No patch needed on clusterserviceversion resource for ignore tag")
		}
	}

	reqLogger.Info("reconcilation complete")
	return reconcile.Result{RequeueAfter: time.Minute * 1}, nil
}

func _csvFilter(metaNew metav1.Object) int {
	ann := metaNew.GetAnnotations()

	//annotation values
	ignoreVal, hasIgnoreTag := ann[ignoreTag]
	_, hasCopiedFrom := ann[olmCopiedFromTag]
	_, hasMeterDefinition := ann[utils.CSV_METERDEFINITION_ANNOTATION]

	switch {
	case hasMeterDefinition && !hasCopiedFrom:
		return 1
	case !hasMeterDefinition && (!hasIgnoreTag || ignoreVal != ignoreTagValue):
		return 2
	default:
	}

	return 0
}

var rhmCSVControllerPredicates predicate.Funcs = predicate.Funcs{
	UpdateFunc: func(evt event.UpdateEvent) bool {
		return csvFilter(evt.MetaNew) > 0
	},
	DeleteFunc: func(evt event.DeleteEvent) bool {
		return true
	},
	CreateFunc: func(evt event.CreateEvent) bool {
		return csvFilter(evt.Meta) > 0
	},
	GenericFunc: func(evt event.GenericEvent) bool {
		return false
	},
}

func (r *RhmCSVReconciler) Inject(injector mktypes.Injectable) mktypes.SetupWithManager {
	injector.SetCustomFields(r)
	return r
}

func (m *RhmCSVReconciler) InjectOperatorConfig(cfg *config.OperatorConfig) error {
	m.cfg = cfg
	return nil
}

func (r *RhmCSVReconciler) SetupWithManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&olmv1alpha1.ClusterServiceVersion{}, builder.WithPredicates(rhmCSVControllerPredicates)).
		Watches(
			&source.Kind{Type: &marketplacev1beta1.MeterDefinition{}}, &handler.EnqueueRequestForOwner{
				IsController: false,
				OwnerType:    &olmv1alpha1.ClusterServiceVersion{},
			}).
		Complete(r)
}
