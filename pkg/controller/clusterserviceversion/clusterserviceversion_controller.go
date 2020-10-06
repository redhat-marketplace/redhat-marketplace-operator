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
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
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
	meterDefStatus  = "marketplace.redhat.com/meterDefinitionStatus"
	meterDefError   = "marketplace.redhat.com/meterDefinitionError"
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
				ignoreVal, ignoreOk := evt.MetaNew.GetAnnotations()[ignoreTag]

				ann := evt.MetaNew.GetAnnotations()
				if ann == nil {
					ann = make(map[string]string)
				}

				_, olmOk := ann["olm.copiedFrom"]
				_, annOk := ann[utils.CSV_METERDEFINITION_ANNOTATION]

				if annOk && !olmOk {
					return true
				}

				if ignoreOk && ignoreVal == "2" {
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
				ann := evt.Meta.GetAnnotations()
				if ann == nil {
					ann = make(map[string]string)
				}

				_, olmOk := ann["olm.copiedFrom"]
				_, annOk := ann[utils.CSV_METERDEFINITION_ANNOTATION]

				if annOk && !olmOk {
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
				ann := evt.Meta.GetAnnotations()
				if ann == nil {
					ann = make(map[string]string)
				}

				_, olmOk := ann["olm.copiedFrom"]
				_, annOk := ann[utils.CSV_METERDEFINITION_ANNOTATION]

				if annOk && !olmOk {
					return true
				}

				return !okAllNamespace
			},
			GenericFunc: func(evt event.GenericEvent) bool {
				ann := evt.Meta.GetAnnotations()
				if ann == nil {
					ann = make(map[string]string)
				}

				_, olmOk := ann["olm.copiedFrom"]
				_, annOk := ann[utils.CSV_METERDEFINITION_ANNOTATION]

				if annOk && !olmOk {
					return true
				}

				return false
			},
		},
	}

	// Watch for changes to primary resource ClusterServiceVersion
	err = c.Watch(&source.Kind{Type: &olmv1alpha1.ClusterServiceVersion{}}, &handler.EnqueueRequestForObject{}, labelPreds...)
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &marketplacev1alpha1.MeterDefinition{}}, &handler.EnqueueRequestForOwner{
		IsController: false,
		OwnerType:    &olmv1alpha1.ClusterServiceVersion{},
	})
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

	// check if CSV is being deleted
	// if yes -> finalizer logic
	// if no -> do nothing
	if !CSV.ObjectMeta.DeletionTimestamp.IsZero() {
		//Run finalization logic for the CSV.
		result, isReconcile, err := r.finalizeCSV(CSV)
		if isReconcile {
			return result, err
		}
	}

	result, isRequeue, err := r.reconcileMeterDefAnnotation(CSV, annotations)

	// check if err is instance of json.parsing error
	// if yes -> add failiure annotation

	if isRequeue {
		return result, err
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
					if len(s.Status.InstalledCSV) == 0 {
						reqLogger.Info("Requeue clusterserviceversion to wait for subscription getting installedCSV updated")
						return reconcile.Result{RequeueAfter: time.Second * 5}, nil
					}

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

		annotations[ignoreTag] = "2"

		if !reflect.DeepEqual(annotations, clusterOriginalAnnotations) {
			CSV.SetAnnotations(annotations)
			if err := r.client.Update(context.TODO(), CSV); err != nil {
				reqLogger.Error(err, "Failed to patch clusterserviceversion ignore tag")
				return reconcile.Result{Requeue: true}, err
			}
			reqLogger.Info("Patched clusterserviceversion with ignore tag")
		} else {
			reqLogger.Info("No patch needed on clusterserviceversion resource for ignore tag")
		}
	}

	reqLogger.Info("reconcilation complete")
	return reconcile.Result{RequeueAfter: time.Minute * 1}, nil
}

func (r *ReconcileClusterServiceVersion) finalizeCSV(CSV *olmv1alpha1.ClusterServiceVersion) (reconcile.Result, bool, error) {
	reqLogger := log.WithValues("Request.Name", CSV.GetName(), "Request.Namespace", CSV.GetNamespace())

	reqLogger.Info("deleting csv")
	if err := r.deleteExternalResources(CSV); err != nil {
		reqLogger.Error(err, "unable to delete csv")
		return reconcile.Result{}, false, err
	}

	// Stop reconciliation as the item is being deleted
	return reconcile.Result{}, true, nil

}

// deleteExternalResources searches for the MeterDefinition created by the CSV, if it's found delete it
func (r *ReconcileClusterServiceVersion) deleteExternalResources(CSV *olmv1alpha1.ClusterServiceVersion) error {
	reqLogger := log.WithValues("Request.Name", CSV.GetName(), "Request.Namespace", CSV.GetNamespace())
	reqLogger.Info("deleting csv")
	var err error

	annotations := CSV.GetAnnotations()
	if annotations == nil {
		reqLogger.Info("No annotations for this CSV")
		return nil
	}

	meterDefinitionString, ok := annotations[utils.CSV_METERDEFINITION_ANNOTATION]
	if !ok {
		reqLogger.Info("No value for ", "key: ", utils.CSV_METERDEFINITION_ANNOTATION)
		return nil
	}

	meterDefinition := &marketplacev1alpha1.MeterDefinition{}
	_, err = meterDefinition.BuildMeterDefinitionFromString(meterDefinitionString, CSV.GetName(), CSV.GetNamespace(), utils.CSV_ANNOTATION_NAME, utils.CSV_ANNOTATION_NAMESPACE)
	if err != nil {
		reqLogger.Error(err, "Could not build a local copy of the MeterDefinition")
		return err
	}

	err = r.client.Delete(context.TODO(), meterDefinition, client.PropagationPolicy(metav1.DeletePropagationForeground))
	if err != nil && errors.IsNotFound(err) {
		return err
	}
	reqLogger.Info("found and deleted MeterDefinition")
	return nil

}

// reconcileMeterDefAnnotation checks the Annotations for the rhm CSV
// If the CSV is new, we tag it and create a MeterDefinition
// If the CSV is old, we check if the actual MeterDefinition matches the CSV (json) MeterDefinition
func (r *ReconcileClusterServiceVersion) reconcileMeterDefAnnotation(CSV *olmv1alpha1.ClusterServiceVersion, annotations map[string]string) (reconcile.Result, bool, error) {
	var err error
	reqLogger := log.WithValues("CSV.Name", CSV.Name, "CSV.Namespace", CSV.Namespace)

	// checks if it is possible to build MeterDefinition from annotations of CSV
	reqLogger.Info("retrieving MeterDefinition string from csv")
	meterDefinitionString, ok := annotations[utils.CSV_METERDEFINITION_ANNOTATION]
	if !ok {
		reqLogger.Info("No value for ", "key: ", utils.CSV_METERDEFINITION_ANNOTATION)
		delete(annotations, meterDefError)
		delete(annotations, meterDefStatus)
		return reconcile.Result{}, false, nil
	}

	// builds a meterdefinition from our string (from the annotation)
	reqLogger.Info("retrieval successful")
	meterDefinition := &marketplacev1alpha1.MeterDefinition{}
	_, err = meterDefinition.BuildMeterDefinitionFromString(
		meterDefinitionString,
		CSV.GetName(),
		CSV.GetNamespace(),
		utils.CSV_ANNOTATION_NAME,
		utils.CSV_ANNOTATION_NAMESPACE)

	if err != nil {
		reqLogger.Error(err, "Could not build a local copy of the MeterDefinition")
		reqLogger.Info("Adding failiure annotation in csv file ")
		annotations[meterDefStatus] = "error"
		annotations[meterDefError] = err.Error()
		CSV.SetAnnotations(annotations)
		if err := r.client.Update(context.TODO(), CSV); err != nil {
			reqLogger.Error(err, "Failed to patch clusterserviceversion with MeterDefinition status")
			return reconcile.Result{}, true, err
		}
		reqLogger.Info("Patched clusterserviceversion with MeterDefinition status")
		return reconcile.Result{}, true, err
	}
	reqLogger.Info("marketplacev1alpha1.MeterDefinitionList >>>> ")
	// Case 1: The CSV is old: compare vs. expected MeterDefinition
	list := &marketplacev1alpha1.MeterDefinitionList{}
	err = r.client.List(context.TODO(), list, client.InNamespace(meterDefinition.GetNamespace()))

	if err != nil {
		reqLogger.Error(err, "Could not retrieve the existing MeterDefinition")
		return reconcile.Result{}, true, err
	}
	reqLogger.Info("marketplacev1alpha1.MeterDefinitionList End --- ")
	var actualMeterDefinition *marketplacev1alpha1.MeterDefinition

	// Find the meterdef, we're use the InstalledBy field
	for _, meterDef := range list.Items {
		if meterDef.Spec.InstalledBy != nil &&
			meterDef.Spec.InstalledBy.Namespace == CSV.Namespace &&
			meterDef.Spec.InstalledBy.Name == CSV.Name {
			actualMeterDefinition = &meterDef
			reqLogger.Info("Found meterDef", "meterDef", meterDef)
			break
		}
	}

	// Check if the name has changed
	if actualMeterDefinition != nil && actualMeterDefinition.Name != meterDefinition.Name {
		reqLogger.Info("Discovered name change", "name", actualMeterDefinition.Name, "newName", meterDefinition.Name)
		err := r.client.Delete(context.TODO(), actualMeterDefinition)

		if err != nil {
			return reconcile.Result{}, true, err
		}

		actualMeterDefinition = nil
	}

	// If not nil, we update
	if actualMeterDefinition != nil {
		if !reflect.DeepEqual(meterDefinition.Spec, actualMeterDefinition.Spec) && !reflect.DeepEqual(meterDefinition.ObjectMeta, actualMeterDefinition.ObjectMeta) {
			reqLogger.Info("The actual meterdefinition is different from the expected meterdefinition")

			patch, err := json.Marshal(meterDefinition)
			if err != nil {
				return reconcile.Result{}, true, err
			}
			err = r.client.Patch(context.TODO(), meterDefinition, client.RawPatch(types.MergePatchType, patch))
			if err != nil {
				return reconcile.Result{Requeue: true}, true, err
			}
			reqLogger.Info("Patch to update MeterDefinition successful. Requeuing")
			return reconcile.Result{Requeue: true}, true, nil
		}
		reqLogger.Info("meter definition matches")
		return reconcile.Result{}, false, nil
	}

	// Case 2: The CSV is new: we must track it & we must create the Meter Definition
	gvk, err := apiutil.GVKForObject(CSV, r.scheme)
	if err != nil {
		return reconcile.Result{}, true, err
	}

	ref := metav1.OwnerReference{
		APIVersion:         gvk.GroupVersion().String(),
		Kind:               gvk.Kind,
		Name:               CSV.GetName(),
		UID:                CSV.GetUID(),
		BlockOwnerDeletion: pointer.BoolPtr(false),
		Controller:         pointer.BoolPtr(false),
	}

	meterDefinition.ObjectMeta.OwnerReferences = append(meterDefinition.ObjectMeta.OwnerReferences, ref)

	if err != nil {
		reqLogger.Error(err, "Failed to create.", "obj", meterDefinition)
		return reconcile.Result{}, true, err
	}

	err = r.client.Create(context.TODO(), meterDefinition)
	if err != nil {
		reqLogger.Error(err, "Could not create MeterDefinition")
		reqLogger.Info("Adding failiure annotation in csv file ")
		annotations[meterDefStatus] = "error"
		annotations[meterDefError] = err.Error()
		CSV.SetAnnotations(annotations)
		if err := r.client.Update(context.TODO(), CSV); err != nil {
			reqLogger.Error(err, "Failed to patch clusterserviceversion with MeterDefinition status")
			return reconcile.Result{}, true, err
		}
		reqLogger.Info("Patched clusterserviceversion with MeterDefinition status")
		return reconcile.Result{}, true, err
	}

	//Add success message annotation to csv
	delete(annotations, meterDefError)
	annotations[meterDefStatus] = "success"
	CSV.SetAnnotations(annotations)
	if err := r.client.Update(context.TODO(), CSV); err != nil {
		reqLogger.Error(err, "Failed to patch clusterserviceversion with MeterDefinition status")
		return reconcile.Result{}, true, err
	}
	reqLogger.Info("Patched clusterserviceversion with MeterDefinition status")

	return reconcile.Result{}, true, nil

}
