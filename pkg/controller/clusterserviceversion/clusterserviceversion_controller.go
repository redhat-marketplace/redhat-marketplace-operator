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
	goerr "errors"
	"reflect"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	utils "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

const (
	operatorTag     = "marketplace.redhat.com/operator"
	watchTag        = "razee/watch-resource"
	allnamespaceTag = "olm.copiedFrom"
	ignoreTag       = "marketplace.redhat.com/ignore"
	trackMeterTag   = "marketplace.redhat.com/track-meter"
)

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
				_, ignoreOk := evt.MetaNew.GetAnnotations()[ignoreTag]

				if strings.Contains(evt.MetaNew.GetName(), utils.CSV_NAME) {
					return true
				}

				if ignoreOk {
					return false
				}

				if okAllNamespace {
					return false
				}

				return !(watchOk && watchLabel == "lite")
			},
			DeleteFunc: func(evt event.DeleteEvent) bool {
				_, okAllNamespace := evt.Meta.GetLabels()[allnamespaceTag]
				watchLabel, watchOk := evt.Meta.GetLabels()[watchTag]
				_, ignoreOk := evt.Meta.GetAnnotations()[ignoreTag]

				if strings.Contains(evt.Meta.GetName(), utils.CSV_NAME) {
					return true
				}

				if ignoreOk {
					return false
				}

				if okAllNamespace {
					return false
				}

				return !(watchOk && watchLabel == "lite")
			},
			CreateFunc: func(evt event.CreateEvent) bool {
				_, okAllNamespace := evt.Meta.GetLabels()[allnamespaceTag]

				if strings.Contains(evt.Meta.GetName(), utils.CSV_NAME) {
					return true
				}

				return !okAllNamespace
			},
			GenericFunc: func(evt event.GenericEvent) bool {
				return strings.Contains(evt.Meta.GetName(), utils.CSV_NAME)
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
	reqLogger := log.WithValues("Request.Name", request.Name, "Request.Namespace", request.Namespace)
	reqLogger.Info("Reconciling ClusterServiceVersion")
	reqLogger.Info("------------------_________________------------------______________-----------------")
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

	//check if the CSV name, matches the RHM-Operator CSV name
	if strings.Contains(CSV.GetName(), utils.CSV_NAME) {
		// examine DeletionTimestamp to determine if object is under deletion
		if CSV.ObjectMeta.DeletionTimestamp.IsZero() {
			// Case 1: the object is not being deleted

			// Check if there is a finalizer attached to CSV
			if !utils.Contains(CSV.GetFinalizers(), utils.CSV_FINALIZER) {
				// if no -> register finalizer
				CSV.ObjectMeta.Finalizers = append(CSV.ObjectMeta.Finalizers, utils.CSV_FINALIZER)
				if err := r.client.Update(context.Background(), CSV); err != nil {
					return reconcile.Result{}, err
				}
				reqLogger.Info("added finailzer to CSV")
			}

			// retrives the string representatin of the MeterDefinition from annotations
			// and builds a MeterDefinition instance out of it
			reqLogger.Info("retrieving MeterDefinition string from csv") // KEEP THIS - FIX WORDING
			meterDefinitionString, ok := annotations[utils.CSV_METERDEFINITION_ANNOTATION]
			if ok {
				reqLogger.Info("retrieval successful")
			} else {
				err = goerr.New("could not retrieve MeterDefinitionString")
				reqLogger.Error(err, "was expecting a an annotation for meterdefinition in CSV")
			}
			reqLogger.Info("building a local copy of MeterDefinition") //FIX WORDING
			meterDefinition := &marketplacev1alpha1.MeterDefinition{}
			_, err = meterDefinition.BuildMeterDefinitionFromString(meterDefinitionString, CSV.GetName(), CSV.GetNamespace(), utils.CSV_ANNOTATION_NAME, utils.CSV_ANNOTATION_NAMESPACE)
			if err != nil {
				reqLogger.Error(err, "Could not build a local copy of the MeterDefinition")
				return reconcile.Result{}, err
			}
			reqLogger.Info("Checking if the csv is being tracked")
			//checks if the CSV is new or old (aka. is it already being tracked)
			if val, ok := annotations[trackMeterTag]; ok && val == "true" {
				// Case 1: The CSV is old: no action required
				reqLogger.Info("CSV is already tracked")
				existingMeterDefinition := &marketplacev1alpha1.MeterDefinition{}
				err = r.client.Get(context.TODO(), client.ObjectKey{Name: meterDefinition.GetName(), Namespace: meterDefinition.GetNamespace()}, existingMeterDefinition)
				if err != nil {
					reqLogger.Error(err, "Could not retrieve the existing MeterDefinition")
					return reconcile.Result{}, err
				}
				if !reflect.DeepEqual(meterDefinition, existingMeterDefinition) {
					err = goerr.New("Existing MeterDefinition and Expected MeterDefinition mismatch")
					reqLogger.Error(err, "The existing meterdefinition is different from the expected meterdefinition")
				} else {
					reqLogger.Info("-------------- METER DEFINITION MATCHES ------------")
				}
			} else {
				reqLogger.Info("csv is new")
				// Case 2: The CSV is new: we must track it & we must create the Meter Definition
				annotations[trackMeterTag] = "true"

				CSV.SetAnnotations(annotations)
				if err := r.client.Update(context.TODO(), CSV); err != nil {
					reqLogger.Error(err, "Failed to patch clusterserviceversion with trackMeter Tag")
					return reconcile.Result{}, err
				}
				reqLogger.Info("Patched clusterserviceversion with trackMeter tag")
				err = r.client.Create(context.TODO(), meterDefinition)
				if err != nil {
					reqLogger.Error(err, "Could not create MeterDefinition")
					return reconcile.Result{}, err
				}

			}
		} else {
			// Case 2: The object is being deleted
			if utils.Contains(CSV.GetFinalizers(), utils.CSV_FINALIZER) {
				// our finalizer is present, so lets handle any external dependency
				reqLogger.Info("deleting csv") // FIX WORDING
				if err := r.deleteExternalResources(CSV); err != nil {
					// if fail to delete the external dependency here, return with error
					// so that it can be retried
					reqLogger.Error(err, "unable to delete csv") // FIX WORDING
					return reconcile.Result{}, err
				}
				// remove our finalizer from the list and update it.
				CSV.SetFinalizers(utils.RemoveKey(CSV.GetFinalizers(), utils.CSV_FINALIZER))
				if err := r.client.Update(context.Background(), CSV); err != nil {
					return reconcile.Result{}, err
				}
			}

			// Stop reconciliation as the item is being deleted
			return reconcile.Result{}, nil
		}
	}

	sub := &olmv1alpha1.SubscriptionList{}

	if err := r.client.List(context.TODO(), sub, client.InNamespace(request.NamespacedName.Namespace)); err != nil {
		return reconcile.Result{}, err
	}

	hasMarketplaceSub := false

	if len(sub.Items) > 0 {
		reqLogger.Info("found Subscription in namespaces", "count", len(sub.Items))
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
	} else {
		reqLogger.Info("Did not find Subscription in namespaces")
	}

	if !hasMarketplaceSub {
		clusterOriginalAnnotations := CSV.DeepCopy().GetAnnotations()

		annotations[ignoreTag] = "true"

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
	return reconcile.Result{RequeueAfter: time.Minute * 1}, nil
}

// deleteExternalResources searches for the MeterDefinition created by the CSV, if it's found delete it
func (r *ReconcileClusterServiceVersion) deleteExternalResources(CSV *olmv1alpha1.ClusterServiceVersion) error {
	reqLogger := log.WithValues("Request.Name", CSV.GetName(), "Request.Namespace", CSV.GetNamespace())
	reqLogger.Info("deleting csv")
	var err error

	meterDefinitionList := &marketplacev1alpha1.MeterDefinitionList{}
	err = r.client.List(context.TODO(), meterDefinitionList, client.InNamespace(CSV.GetNamespace()))
	if err != nil {
		return err
	}

	// search specifically for the Meter Definition created by the CSV
	for _, meterDefinition := range meterDefinitionList.Items {
		ann := meterDefinition.GetAnnotations()
		if _, ok := ann[utils.CSV_ANNOTATION_NAME]; ok {
			// CSV specific MeterDefinition found; delete it
			err = r.client.Delete(context.TODO(), &meterDefinition, client.PropagationPolicy(metav1.DeletePropagationForeground))
			if err != nil {
				return err
			} else {
				reqLogger.Info("found and deleted MeterDefinition")
				return nil
			}
		}
	}

	reqLogger.Info("no meterdefinition found")
	return nil
}
