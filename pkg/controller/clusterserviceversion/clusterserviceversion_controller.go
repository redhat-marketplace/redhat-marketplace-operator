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
	goerr "errors"
	"io/ioutil"
	"reflect"
	"strings"

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
	IgnoreTag       = "marketplace.redhat.com/ignore"
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
				// _, okAllNamespace := evt.MetaNew.GetLabels()[allnamespaceTag]
				// watchLabel, watchOk := evt.MetaNew.GetLabels()[watchTag]
				// _, ignoreOk := evt.MetaNew.GetAnnotations()[IgnoreTag]

				// if ignoreOk {
				// 	return false
				// }

				// if okAllNamespace {
				// 	return false
				// }

				// return !(watchOk && watchLabel == "lite")
				return strings.Contains(evt.MetaNew.GetName(), utils.CSV_NAME)
				// return true
			},
			DeleteFunc: func(evt event.DeleteEvent) bool {
				// _, okAllNamespace := evt.Meta.GetLabels()[allnamespaceTag]
				// watchLabel, watchOk := evt.Meta.GetLabels()[watchTag]
				// _, ignoreOk := evt.Meta.GetAnnotations()[IgnoreTag]

				// if ignoreOk {
				// 	return false
				// }

				// if okAllNamespace {
				// 	return false
				// }

				// return !(watchOk && watchLabel == "lite")
				return strings.Contains(evt.Meta.GetName(), utils.CSV_NAME)
				// return true
			},
			CreateFunc: func(evt event.CreateEvent) bool {
				// _, okAllNamespace := evt.Meta.GetLabels()[allnamespaceTag]
				// return !okAllNamespace
				return strings.Contains(evt.Meta.GetName(), utils.CSV_NAME)
				// return true
			},
			GenericFunc: func(evt event.GenericEvent) bool {
				// return false
				return strings.Contains(evt.Meta.GetName(), utils.CSV_NAME)
				// return true
			},
		},
	}

	// Watch for changes to primary resource ClusterServiceVersion
	err = c.Watch(&source.Kind{Type: &olmv1alpha1.ClusterServiceVersion{}}, &handler.EnqueueRequestForObject{}, labelPreds...)
	if err != nil {
		return err
	}
	return nil

	p := []predicate.Predicate{
		predicate.Funcs{
			CreateFunc: func(evt event.CreateEvent) bool {
				return false
			},
			UpdateFunc: func(evt event.UpdateEvent) bool {
				ann := evt.MetaOld.GetAnnotations()
				if _, ok := ann["csvName"]; ok {
					return true
				}
				return false
			},
			DeleteFunc: func(evt event.DeleteEvent) bool {
				ann := evt.Meta.GetAnnotations()
				if _, ok := ann["csvName"]; ok {
					return true
				}
				return false
			},
			GenericFunc: func(evt event.GenericEvent) bool {
				ann := evt.Meta.GetAnnotations()
				if _, ok := ann["csvName"]; ok {
					return true
				}
				return false
			},
		},
	}
	err = c.Watch(&source.Kind{Type: &marketplacev1alpha1.MeterDefinition{}}, &handler.EnqueueRequestForObject{}, p...)
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

	if strings.Contains(CSV.GetName(), utils.CSV_NAME) {
		// examine DeletionTimestamp to determine if object is under deletion
		if CSV.ObjectMeta.DeletionTimestamp.IsZero() {

			// The object is not being deleted, so if it does not have our finalizer,
			// then lets add the finalizer and update the object. This is equivalent
			// registering our finalizer.
			if !utils.Contains(CSV.GetFinalizers(), utils.CSV_FINALIZER) {
				CSV.ObjectMeta.Finalizers = append(CSV.ObjectMeta.Finalizers, utils.CSV_FINALIZER)
				if err := r.client.Update(context.Background(), CSV); err != nil {
					return reconcile.Result{}, err
				} else {
					reqLogger.Info("-------------Added finalizer-------------")
				}

				reqLogger.Info("-------------FOUND: CSV, NOW CHECK ANNOTATIONS-------------")
				// retrives the string representatino of the MeterDefinition from annotations
				// and builds a MeterDefinition instance out of it
				reqLogger.Info("-------------Retrieving MeterDefinition String-------------")
				meterDefinitionString, err := getMeterDefinitionString(annotations["alm-example"])
				if err != nil {
					reqLogger.Error(err, "Failed to retrieve the MeterDefinition String for this CSV")
					return reconcile.Result{}, err
				}
				reqLogger.Info("-------------Building a copy of MeterDefinition-------------")
				meterDefinition := &marketplacev1alpha1.MeterDefinition{}
				_, err = meterDefinition.BuildMeterDefinitionFromString(meterDefinitionString, CSV.GetName(), CSV.GetNamespace())
				if err != nil {
					reqLogger.Error(err, "Could not build a copy of the MeterDefinition")
					return reconcile.Result{}, err
				}
				reqLogger.Info("-------------NOW CHECK ANNOTATIONS-------------")
				if val, ok := annotations[trackMeterTag]; ok && val == "true" {
					// The CSV has already been created, and we are already tracking

					//CSV is not new
					//In this case: compare the existing MeterDefinition, with whats in the annotations
					reqLogger.Info("-------------CSV already tracked-------------")
					existingMeterDefinition := &marketplacev1alpha1.MeterDefinition{}
					err = r.client.Get(context.TODO(), client.ObjectKey{Name: meterDefinition.GetName(), Namespace: meterDefinition.GetNamespace()}, existingMeterDefinition)
					if err != nil {
						reqLogger.Error(err, "Could not retrieve the existing MeterDefinition")
						return reconcile.Result{}, err
					}
					if !reflect.DeepEqual(meterDefinition, existingMeterDefinition) {
						err = goerr.New("Existing MeterDefinition and Expected MeterDefinition mismatch")
						reqLogger.Error(err, "The existing meterdefinition is different from the expected meterdefinition")
					}
				} else {
					// The CSV is new, we must track it and create the Meter Definition
					annotations[trackMeterTag] = "true"

					CSV.SetAnnotations(annotations)
					if err := r.client.Update(context.TODO(), CSV); err != nil {
						reqLogger.Error(err, "Failed to patch clusterserviceversion trackMeter Tag")
						return reconcile.Result{}, err
					}
					reqLogger.Info("Patched clusterserviceversion with trackMeter tag")
					//CSV is new
					//In this case: create the MeterDefinition
					reqLogger.Info("-------------CSV IS NEW-------------")
					err = r.client.Create(context.TODO(), meterDefinition)
					if err != nil {
						reqLogger.Error(err, "Could not create MeterDefinition")
						return reconcile.Result{}, err
					}

				}
			}
		} else {
			// The object is being deleted
			reqLogger.Info("-------------CSV Finalizer Name: -------------", "name: ", strings.Join(CSV.GetFinalizers(), ", "), "coded name: ", utils.CSV_FINALIZER)
			if utils.Contains(CSV.GetFinalizers(), utils.CSV_FINALIZER) {
				// our finalizer is present, so lets handle any external dependency
				reqLogger.Info("-------------Deleting CSV-------------")
				if err := r.deleteExternalResources(CSV); err != nil {
					// if fail to delete the external dependency here, return with error
					// so that it can be retried
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

func getMeterDefinitionString(almExample string) (string, error) {
	var err error
	var objs []runtime.Unstructured
	var result string

	data := []byte(almExample)

	err = json.Unmarshal(data, &objs)
	if err != nil {
		return "", err
	}

	b, err := json.Marshal(objs)
	err = ioutil.WriteFile("after", b, 0644)
	if err != nil {
		return "", err
	}

	for _, obj := range objs {
		if obj.GetObjectKind().GroupVersionKind().Kind == "MeterDefinition" {
			res, err := json.Marshal(obj)
			if err != nil {
				return "", err
			}
			result = string(res)
			return result, nil
		}
	}

	err = goerr.New("Could not find a kind MeterDefinition")
	return result, err
}

// deleteExternalResources searches for the MeterDefinition created by the CSV, if it's found delete it
func (r *ReconcileClusterServiceVersion) deleteExternalResources(CSV *olmv1alpha1.ClusterServiceVersion) error {
	reqLogger := log.WithValues("Request.Name", CSV.GetName(), "Request.Namespace", CSV.GetNamespace())
	reqLogger.Info("-------------DELETING CSV-------------")
	var err error

	meterDefinitionList := &marketplacev1alpha1.MeterDefinitionList{}
	err = r.client.List(context.TODO(), meterDefinitionList, client.InNamespace(CSV.GetNamespace()))
	if err != nil {
		return err
	}

	for _, meterDefinition := range meterDefinitionList.Items {
		ann := meterDefinition.GetAnnotations()
		if _, ok := ann["csvName"]; ok {
			err = r.client.Delete(context.TODO(), &meterDefinition, client.PropagationPolicy(metav1.DeletePropagationForeground))
			if err != nil {
				return err
			} else {
				reqLogger.Info("FOUND AND DELETED METER DEFINITION")
				return nil
			}
		}
	}

	reqLogger.Info("NO METER DEFINITION FOUND")
	return nil
}
