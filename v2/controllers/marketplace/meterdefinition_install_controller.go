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
	"strings"
	"time"

	"github.com/go-logr/logr"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/catalog"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
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

// blank assignment to verify that ReconcileClusterServiceVersion implements reconcile.Reconciler
var _ reconcile.Reconciler = &MeterdefinitionInstallReconciler{}

// MeterdefinitionInstallReconciler reconciles a ClusterServiceVersion object
type MeterdefinitionInstallReconciler struct {
	// This Client, initialized using mgr.Client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	Client        client.Client
	Scheme        *runtime.Scheme
	Log           logr.Logger
	cfg           *config.OperatorConfig
	kubeInterface kubernetes.Interface
}

// +kubebuilder:rbac:groups="operators.coreos.com",resources=clusterserviceversions;subscriptions,verbs=get;list;watch
// +kubebuilder:rbac:groups="operators.coreos.com",resources=clusterserviceversions,verbs=update;patch
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=meterdefinitions;meterdefinitions/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=meterdefinitions;meterdefinitions/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="operators.coreos.com",resources=clusterserviceversions;subscriptions,verbs=get;list;watch
// +kubebuilder:rbac:urls=/list-for-version/*,verbs=get;
// +kubebuilder:rbac:urls=/get-system-meterdefs/*,verbs=get;
// +kubebuilder:rbac:groups="authentication.k8s.io",resources=tokenreviews,verbs=create;get
// +kubebuilder:rbac:groups="authorization.k8s.io",resources=subjectaccessreviews,verbs=create;get

// Reconcile reads that state of the cluster for a ClusterServiceVersion object and creates corresponding meter definitions if found
func (r *MeterdefinitionInstallReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Name", request.Name, "Request.Namespace", request.Namespace)
	reqLogger.Info("Reconciling ClusterServiceVersion")

	// Fetch the ClusterServiceVersion instance
	CSV := &olmv1alpha1.ClusterServiceVersion{}
	err := r.Client.Get(context.TODO(), request.NamespacedName, CSV)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, check the meterdef store if there is an existing InstallMapping,delete, and return empty result
			reqLogger.Info("clusterserviceversion does not exist", "name", request.Name)
			return reconcile.Result{}, nil
		}

		reqLogger.Error(err, "Failed to get clusterserviceversion")
		return reconcile.Result{}, err
	}

	csvName := strings.Split(CSV.Name, ".")[0]
	reqLogger.Info("csv name", "name", csvName)
	csvVersion := CSV.Spec.Version.Version.String()
	reqLogger.Info("csv version", "version", csvVersion)

	// New CSV install
	sub := &olmv1alpha1.SubscriptionList{}
	if err := r.Client.List(context.TODO(), sub, client.InNamespace(request.NamespacedName.Namespace)); err != nil {
		return reconcile.Result{}, err
	}

	_csvName := strings.Split(request.Name, ".")[0]
	var foundSub *olmv1alpha1.Subscription
	if len(sub.Items) > 0 {
		for _, s := range sub.Items {
			if strings.HasPrefix(s.Name, _csvName) {
				foundSub = &s
			}
		}
	}

	if foundSub != nil {
		reqLogger.Info("found Subscription in namespaces", "count", len(sub.Items))

		if value, ok := foundSub.GetLabels()[operatorTag]; ok {

			if value == "true" {
				if len(foundSub.Status.InstalledCSV) == 0 {
					reqLogger.Info("Requeue clusterserviceversion to wait for subscription getting")
					return reconcile.Result{RequeueAfter: time.Second * 5}, nil
				}

				if foundSub.Status.InstalledCSV != request.NamespacedName.Name {
					return reconcile.Result{RequeueAfter: time.Second * 5}, nil
				}

				if foundSub.Status.InstalledCSV == request.NamespacedName.Name {
					reqLogger.Info("found Subscription with installed CSV")

					catalogClient, err := catalog.NewCatalogClientBuilder(r.cfg).NewCatalogServerClient(r.Client,r.cfg.DeployedNamespace,r.kubeInterface,reqLogger)
					if err != nil {
						return reconcile.Result{}, err
					}

					_, selectedMeterDefinitions, result := catalogClient.ListMeterdefintionsFromFileServer(csvName, csvVersion, CSV.Namespace,reqLogger)
					if !result.Is(Continue) {

						if result.Is(Error) {
							reqLogger.Error(result.GetError(), "Failed retrieving meterdefinitions from file server", "CSV", csvName)
						}

						return result.Return()
					}

					_, globalMeterdefinitions, result := catalogClient.GetSystemMeterdefs(csvName, csvVersion, CSV.Namespace, reqLogger)
					if !result.Is(Continue) {

						if result.Is(Error) {
							reqLogger.Error(result.GetError(), "Failed retrieving global meterdefinitions", "CSV", csvName)
						}

						result.Return()
					}

					allMeterDefinitions := append(globalMeterdefinitions, selectedMeterDefinitions...)

					gvk, err := apiutil.GVKForObject(CSV, r.Scheme)
					if err != nil {
						return reconcile.Result{}, err
					}

					// create CSV specific and global meter definitions
					reqLogger.Info("creating meterdefinitions", "CSV", csvName)
					for _, meterDefItem := range allMeterDefinitions {
						reqLogger.Info("checking for existing meterdefinition", "meterdef", meterDefItem.Name, "CSV", csvName)

						// Check if the meterdef is on the cluster already
						meterdef := &marketplacev1beta1.MeterDefinition{}
						err = r.Client.Get(context.TODO(), types.NamespacedName{Name: meterDefItem.Name, Namespace: request.Namespace}, meterdef)
						if err != nil {
							if errors.IsNotFound(err) {
								reqLogger.Info("meterdefinition not found, creating", "meterdef name", meterDefItem.Name, "CSV", CSV.Name)

								result = r.createMeterdef(csvName, csvVersion, meterDefItem, CSV, gvk, request, reqLogger)
								if !result.Is(Continue) {

									if result.Is(Error) {
										reqLogger.Error(result.GetError(), "Failed while creating meterdefinition", "meterdef name", meterDefItem.Name, "CSV", CSV.Name)
									}
									return result.Return()
								}

								reqLogger.Info("created meterdefinition", "meterdef name", meterDefItem.Name, "CSV", CSV.Name)
								return reconcile.Result{Requeue: true}, nil
							}

							reqLogger.Error(err, "Failed to get meterdefinition", "meterdef name", meterDefItem.Name, "CSV", CSV.Name)
							return reconcile.Result{}, err
						}
					}
				}
			}
		}
	} else {
		reqLogger.Info("Subscriptions not found in the namespace")
		return reconcile.Result{RequeueAfter: time.Minute * 1}, nil
	}

	reqLogger.Info("reconcilation complete")
	return reconcile.Result{}, nil
}

//TODO: remove this and just pick up every CSV
// func reconcileCSV(metaNew metav1.Object) bool {
// 	ann := metaNew.GetAnnotations()

// 	ignoreVal, hasIgnoreTag := ann[ignoreTag]

// 	// we need to pick up the csv
// 	if !hasIgnoreTag || ignoreVal != ignoreTagValue {
// 		return true
// 	}

// 	//ignore
// 	return false
// }

func (r *MeterdefinitionInstallReconciler) createMeterdef(csvName string, csvVersion string, meterDefinition marketplacev1beta1.MeterDefinition, csv *olmv1alpha1.ClusterServiceVersion, groupVersionKind schema.GroupVersionKind, request reconcile.Request, reqLogger logr.InfoLogger) *ExecResult {

	// create owner ref object
	ref := metav1.OwnerReference{
		APIVersion:         groupVersionKind.GroupVersion().String(),
		Kind:               groupVersionKind.Kind,
		Name:               csv.GetName(),
		UID:                csv.GetUID(),
		BlockOwnerDeletion: pointer.BoolPtr(false),
		Controller:         pointer.BoolPtr(false),
	}

	meterDefinition.ObjectMeta.SetOwnerReferences([]metav1.OwnerReference{ref})
	meterDefinition.ObjectMeta.Namespace = csv.Namespace

	meterDefName := meterDefinition.Name
	err := r.Client.Create(context.TODO(), &meterDefinition)
	if err != nil {
		reqLogger.Error(err, "Could not create meterdefinition", "mdef", meterDefName, "CSV", csv.Name)
		return &ExecResult{
			ReconcileResult: reconcile.Result{},
			Err:             err,
		}
	}
	reqLogger.Info("Created meterdefinition", "mdef", meterDefName, "CSV", csv.Name)

	return &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}

func checkForCSVVersionChanges(e event.UpdateEvent) bool {
	oldCSV, ok := e.ObjectOld.(*olmv1alpha1.ClusterServiceVersion)
	if !ok {
		return false
	}

	newCSV, ok := e.ObjectNew.(*olmv1alpha1.ClusterServiceVersion)
	if !ok {
		return false
	}

	return oldCSV.Spec.Version.String() != newCSV.Spec.Version.String()
}

var rhmCSVControllerPredicates predicate.Funcs = predicate.Funcs {
	UpdateFunc: func(e event.UpdateEvent) bool {
		// if !reconcileCSV(e.MetaNew) || !reconcileCSV(e.MetaOld) {
		// 	return false
		// }
		return checkForCSVVersionChanges(e)
	},

	DeleteFunc: func(e event.DeleteEvent) bool {
		// return reconcileCSV(e.Meta)
		return true
	},

	CreateFunc: func(e event.CreateEvent) bool {
		// return reconcileCSV(e.Meta)
		return true

	},

	GenericFunc: func(e event.GenericEvent) bool {
		// return reconcileCSV(e.Meta)
		return true
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

func (r *MeterdefinitionInstallReconciler) InjectKubeInterface(k kubernetes.Interface) error {
	r.kubeInterface = k
	return nil
}

func (r *MeterdefinitionInstallReconciler) SetupWithManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&olmv1alpha1.ClusterServiceVersion{}, builder.WithPredicates(rhmCSVControllerPredicates)).
		Complete(r)
}
