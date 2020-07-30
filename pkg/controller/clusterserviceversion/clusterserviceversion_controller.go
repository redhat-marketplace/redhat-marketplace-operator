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

package clusterserviceversion

import (
	"context"
	"encoding/json"
	"reflect"

	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_olm_clusterserviceversion_watcher")

const operatorTag = "marketplace.redhat.com/operator"
const watchTag = "razee/watch-resource"
const allnamespaceTag = "olm.copiedFrom"
const IgnoreTag = "marketplace.redhat.com/ignore"

// Add creates a new ClusterServiceVersion Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileClusterServiceVersion{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("clusterserviceversion-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	labelPreds := []predicate.Predicate{
		predicate.Funcs{
			UpdateFunc: func(evt event.UpdateEvent) bool {
				_, okAllNamespace := evt.MetaNew.GetLabels()[allnamespaceTag]
				watchLabel, watchOk := evt.MetaNew.GetLabels()[watchTag]
				_, ignoreOk := evt.MetaNew.GetAnnotations()[IgnoreTag]

				if ignoreOk {
					return false
				}

				if okAllNamespace {
					return false
				}

				return !(watchOk && watchLabel == "lite")
			},
			DeleteFunc: func(evt event.DeleteEvent) bool {
				return false
			},
			CreateFunc: func(evt event.CreateEvent) bool {
				_, okAllNamespace := evt.Meta.GetLabels()[allnamespaceTag]
				return !okAllNamespace
			},
			GenericFunc: func(evt event.GenericEvent) bool {
				return false
			},
		},
	}

	// Watch for changes to primary resource ClusterServiceVersion
	err = c.Watch(&source.Kind{Type: &olmv1alpha1.ClusterServiceVersion{}}, &handler.EnqueueRequestForObject{}, labelPreds...)
	if err != nil {
		return err
	}
	return nil
}

// blank assignment to verify that ReconcileClusterServiceVersion implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileClusterServiceVersion{}

// ReconcileClusterServiceVersion reconciles a ClusterServiceVersion object
type ReconcileClusterServiceVersion struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a ClusterServiceVersion object and makes changes based on the state read
// and what is in the ClusterServiceVersion.Spec
func (r *ReconcileClusterServiceVersion) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Name", request.Name)
	reqLogger.Info("Reconciling ClusterServiceVersion")
	// Fetch the ClusterServiceVersion instance
	CSV := &olmv1alpha1.ClusterServiceVersion{}
	err := r.client.Get(context.TODO(), request.NamespacedName, CSV)

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

	if err := r.client.List(context.TODO(), sub, client.InNamespace(request.NamespacedName.Namespace)); err != nil {
		return reconcile.Result{}, err
	}

	reqLogger.Info("found Subscription in namespaces", "count", len(sub.Items))
	hasMarketplaceSub := false

	if len(sub.Items) > 0 {
		// add razee watch label to CSV if subscription has rhm/operator label
		for _, s := range sub.Items {
			if value, ok := s.GetLabels()[operatorTag]; ok {
				if value == "true" {
					if s.Status.InstalledCSV == request.NamespacedName.Name {
						reqLogger.Info("found Subscription with installed CSV")

						hasMarketplaceSub = true

						labels := CSV.GetLabels()
						clusterOriginalLabels := CSV.DeepCopy().GetLabels()
						if labels == nil {
							labels = make(map[string]string)
						}

						labels[watchTag] = "lite"

						if !reflect.DeepEqual(labels, clusterOriginalLabels) {
							CSV.SetLabels(labels)
							if err := r.client.Update(context.TODO(), CSV); err != nil {
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
	}

	if !hasMarketplaceSub {
		clusterOriginalAnnotations := CSV.DeepCopy().GetAnnotations()

		annotations[IgnoreTag] = "true"

		if !reflect.DeepEqual(annotations, clusterOriginalAnnotations) {
			CSV.SetAnnotations(annotations)
			if err := r.client.Update(context.TODO(), CSV); err != nil {
				reqLogger.Error(err, "Failed to patch clusterserviceversion ignore tag")
				return reconcile.Result{}, err
			}
			reqLogger.Info("Patched clusterserviceversion with ignore tag")
		} else {
			reqLogger.Info("No patch needed on clusterserviceversion resource for ignore tag")
		}
	}

	reqLogger.Info("reconcilation complete")
	return reconcile.Result{}, nil
}

func buildMeterDefCR(meterdefString, namespace string) (*marketplacev1alpha1.MeterDefinition, error) {
	var meterdef *marketplacev1alpha1.MeterDefinition
	data := []byte(meterdefString)
	err := json.Unmarshal(data, meterdef)
	if err != nil {
		return meterdef, err
	}
	meterdef.Namespace = namespace
	return meterdef, nil
}
