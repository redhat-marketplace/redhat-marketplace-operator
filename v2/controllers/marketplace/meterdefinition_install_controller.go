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
	"encoding/json"
	"time"

	semver "github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	csvProp      string = "operatorframework.io/properties"
	versionRange string = "versionRange"
	packageName  string = "packageName"
	startVersion string = "startVersion"
)

// blank assignment to verify that ReconcileClusterServiceVersion implements reconcile.Reconciler
var _ reconcile.Reconciler = &MeterdefinitionInstallReconciler{}

// MeterdefinitionInstallReconciler reconciles a ClusterServiceVersion object
type MeterdefinitionInstallReconciler struct {
	// This Client, initialized using mgr.Client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	Client client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
	cfg    *config.OperatorConfig
}

// +kubebuilder:rbac:groups="operators.coreos.com",resources=clusterserviceversions;subscriptions,verbs=get;list;watch
// +kubebuilder:rbac:groups="operators.coreos.com",resources=clusterserviceversions,verbs=update;patch
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=meterdefinitions;meterdefinitions/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=meterdefinitions;meterdefinitions/status,verbs=get;list;watch;create;update;patch;delete

// Reconcile reads that state of the cluster for a ClusterServiceVersion object and creates corresponding meter definitions if found
func (r *MeterdefinitionInstallReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
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

	sub := &olmv1alpha1.SubscriptionList{}
	if err := r.Client.List(context.TODO(), sub, client.InNamespace(request.NamespacedName.Namespace)); err != nil {
		return reconcile.Result{}, err
	}

	if len(sub.Items) > 0 {
		reqLogger.V(4).Info("found Subscription in namespaces", "count", len(sub.Items))

		// apply meter definition if subscription has rhm/operator label
		for _, s := range sub.Items {
			if value, ok := s.GetLabels()[operatorTag]; ok {
				if value == "true" {
					if len(s.Status.InstalledCSV) == 0 {
						reqLogger.Info("Requeue clusterserviceversion to wait for subscription getting installedCSV updated")
						return reconcile.Result{RequeueAfter: time.Second * 5}, nil
					}

					if s.Status.InstalledCSV == request.NamespacedName.Name {
						reqLogger.Info("found Subscription with installed CSV")

						v, ok := CSV.GetAnnotations()[csvProp]
						if !ok {
							reqLogger.Error(err, "annotation for CSV properties was not found")
							return reconcile.Result{}, err
						}
						csvProperties := fetchCSVInfo(v)

						packageName := csvProperties["packageName"]
						version := csvProperties["version"]

						// get all the meter definitions to be created
						var selectedMeterDefinitions = []marketplacev1beta1.MeterDefinition{}
						for _, meterDefinition := range GlobalMeterdefStoreDB.ListMeterdefinitions() {

							if checkMeterDefinition(packageName.(string), version.(string), meterDefinition, reqLogger) {
								selectedMeterDefinitions = append(selectedMeterDefinitions, meterDefinition)
							}
						}

						// create meter definitions
						for _, meterDefItem := range selectedMeterDefinitions {
							
							gvk, err := apiutil.GVKForObject(CSV, r.Scheme)
							if err != nil {
								return reconcile.Result{}, err
							}

							ref := metav1.OwnerReference{
								APIVersion:         gvk.GroupVersion().String(),
								Kind:               gvk.Kind,
								Name:               CSV.GetName(),
								UID:                CSV.GetUID(),
								BlockOwnerDeletion: pointer.BoolPtr(false),
								Controller:         pointer.BoolPtr(false),
							}

							ownerRef := meterDefItem.ObjectMeta.OwnerReferences
							if ownerRef != nil {
								meterDefItem.ObjectMeta.OwnerReferences = append(meterDefItem.ObjectMeta.OwnerReferences, ref)
							} else {
								meterDefItem.ObjectMeta.SetOwnerReferences([]metav1.OwnerReference{ref})
							}

							meterDefItem.ObjectMeta.Namespace = CSV.Namespace

							// Fetch the mdefKVStore instance
							meterdef := &marketplacev1beta1.MeterDefinition{}
							err = r.Client.Get(context.TODO(), types.NamespacedName{Name: meterDefItem.Name,Namespace: request.Namespace}, meterdef)
							if err != nil {
								if errors.IsNotFound(err) {
									err = r.Client.Create(context.TODO(), &meterDefItem)
									if err != nil {
										reqLogger.Error(err, "Could not create MeterDefinition", "mdef", &meterDefItem.Name)
										return reconcile.Result{}, err
									}
									
									return reconcile.Result{Requeue: true}, nil
								}

								reqLogger.Error(err, "Failed to get meterdefinition")
								return reconcile.Result{}, err
							}

							reqLogger.Info("Created meter definition", "mdef", &meterDefItem.Name)
						}
					}
				}
			}
		}
	} else {
		reqLogger.Info("Did not find Subscription in namespaces")
	}

	reqLogger.Info("reconcilation complete")
	return reconcile.Result{RequeueAfter: time.Minute * 1}, nil
}

func _csvFilter(metaNew metav1.Object) int {
	ann := metaNew.GetAnnotations()

	//annotation values
	ignoreVal, hasIgnoreTag := ann[ignoreTag]

	if !hasIgnoreTag || ignoreVal != ignoreTagValue {
		return 1
	}
	return 0
}

func checkMeterDefinition(csvPackageName string, version string, meterDefinition marketplacev1beta1.MeterDefinition, reqLogger logr.Logger) bool {
	meterdefVersionRange := meterDefinition.GetAnnotations()[versionRange]
	meterdefPackageName := meterDefinition.GetAnnotations()[packageName]
	
	reqLogger.Info("version range from meterdef","range",meterdefVersionRange)
	reqLogger.Info("version from CSV", "version", version)
	reqLogger.Info("package name of CSV","package name",csvPackageName)
	reqLogger.Info("package name from meterdef","name",meterdefPackageName)

	if csvPackageName == meterdefPackageName {
		meterdefVersionConstraint, err := semver.NewConstraint(meterdefVersionRange)
		if err != nil {
			reqLogger.Error(err,"error setting up constraint")
			return false
		}

		csvVersion, err := semver.NewVersion(version)
		if err != nil {
			reqLogger.Error(err,"error creating version","version",version)
		}

		// Check if the version meets the constraints. The a variable will be true.
		validVersion := meterdefVersionConstraint.Check(csvVersion)
		if validVersion {
			reqLogger.Info("version is valid")
		}
		return validVersion
	
	}
	
	return false
}

func fetchCSVInfo(csvProps string) map[string]interface{} {
	var unmarshalledProps map[string]interface{}
	json.Unmarshal([]byte(csvProps), &unmarshalledProps)

	properties := unmarshalledProps["properties"].([]interface{})
	reqProperty := properties[1]

	csvProperty := reqProperty.(map[string]interface{})
	return csvProperty["value"].(map[string]interface{})
}

var rhmCSVControllerPredicates predicate.Funcs = predicate.Funcs{
	UpdateFunc: func(e event.UpdateEvent) bool {
		if e.MetaNew.GetName() == "" {
			return e.MetaOld.GetResourceVersion() != e.MetaNew.GetResourceVersion()
		}
		return false
	},
	DeleteFunc: func(evt event.DeleteEvent) bool {
		return false
	},
	CreateFunc: func(evt event.CreateEvent) bool {
		return csvFilter(evt.Meta) > 0
	},
	GenericFunc: func(evt event.GenericEvent) bool {
		return false
	},
}

func (r *MeterdefinitionInstallReconciler) Inject(injector mktypes.Injectable) mktypes.SetupWithManager {
	injector.SetCustomFields(r)
	return r
}

func (m *MeterdefinitionInstallReconciler) InjectOperatorConfig(cfg *config.OperatorConfig) error {
	m.cfg = cfg
	return nil
}

func (r *MeterdefinitionInstallReconciler) SetupWithManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&olmv1alpha1.ClusterServiceVersion{}, builder.WithPredicates(rhmCSVControllerPredicates)).
		Complete(r)
}
