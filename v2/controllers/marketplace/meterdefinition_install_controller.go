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
	"errors"
	"fmt"
	"time"

	emperrors "emperror.dev/errors"
	"github.com/go-logr/logr"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"

	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/catalog"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/matcher"

	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
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

// blank assignment to verify that ReconcileClusterServiceVersion implements reconcile.Reconciler
var _ reconcile.Reconciler = &MeterDefinitionInstallReconciler{}

// MeterDefinitionInstallReconciler reconciles a ClusterServiceVersion object
type MeterDefinitionInstallReconciler struct {
	// This Client, initialized using mgr.Client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	Client        client.Client
	Scheme        *runtime.Scheme
	Log           logr.Logger
	cfg           *config.OperatorConfig
	catalogClient *catalog.CatalogClient
}

// +kubebuilder:rbac:groups="operators.coreos.com",resources=clusterserviceversions;subscriptions,verbs=get;list;watch
// +kubebuilder:rbac:groups="operators.coreos.com",resources=clusterserviceversions,verbs=update;patch
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=meterdefinitions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=meterdefinitions;meterdefinitions/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=meterdefinitions;meterdefinitions/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="operators.coreos.com",resources=clusterserviceversions;subscriptions,verbs=get;list;watch
// +kubebuilder:rbac:urls=/list-for-version/*,verbs=get;
// +kubebuilder:rbac:urls=/get-system-meterdefs,verbs=get;post;create;
// +kubebuilder:rbac:groups="authentication.k8s.io",resources=tokenreviews,verbs=create;get
// +kubebuilder:rbac:groups="authorization.k8s.io",resources=subjectaccessreviews,verbs=create;get

// Reconcile reads that state of the cluster for a ClusterServiceVersion object and creates corresponding meter definitions if found
func (r *MeterDefinitionInstallReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Name", request.Name, "Request.Namespace", request.Namespace)
	reqLogger.Info("Reconciling ClusterServiceVersion")

	instance := &marketplacev1alpha1.MeterBase{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: utils.METERBASE_NAME, Namespace: r.cfg.DeployedNamespace}, instance)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			reqLogger.Error(err, "meterbase does not exist must have been deleted - ignoring for now")
			return reconcile.Result{}, nil
		}

		reqLogger.Error(err, "Failed to get meterbase")
		return reconcile.Result{}, err
	}

	if instance.Spec.MeterdefinitionCatalogServerConfig == nil {
		reqLogger.Info("meterbase doesn't have file server feature flags set")
		return reconcile.Result{}, nil
	}

	// catalog server not enabled, stop reconciling
	if !instance.Spec.MeterdefinitionCatalogServerConfig.DeployMeterDefinitionCatalogServer {
		reqLogger.Info("catalog server isn't enabled, stopping reconcile")
		return reconcile.Result{}, nil
	}

	// Fetch the ClusterServiceVersion instance
	CSV := &olmv1alpha1.ClusterServiceVersion{}
	err = r.Client.Get(context.TODO(), request.NamespacedName, CSV)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Request object not found, check the meterdef store if there is an existing InstallMapping,delete, and return empty result
			reqLogger.Info("clusterserviceversion does not exist", "name", request.Name)
			return reconcile.Result{}, nil
		}

		reqLogger.Error(err, "Failed to get clusterserviceversion")
		return reconcile.Result{}, err
	}

	csvName := CSV.Name
	reqLogger.Info("csv name", "name", csvName)

	csvVersion := CSV.Spec.Version.Version.String()
	reqLogger.Info("csv version", "version", csvVersion)

	subList := &olmv1alpha1.SubscriptionList{}
	if err := r.Client.List(context.TODO(), subList, client.InNamespace(CSV.Namespace)); err != nil {
		return reconcile.Result{}, err
	}

	packageName,err := matcher.ParsePackageName(CSV)
	if err != nil {
		reqLogger.Info(err.Error())
	}

	foundSub,err := matcher.MatchCsvToSub(r.cfg.ControllerValues.RhmCatalogName,packageName,subList.Items,CSV)
	if errors.Is(err,matcher.ErrSubscriptionIsUpdating){
		reqLogger.Info(err.Error())
		return reconcile.Result{RequeueAfter: time.Second * 5},nil
	}

	isRHMSub := matcher.CheckOperatorTag(foundSub)
	if isRHMSub {
		reqLogger.Info("found Subscription with installed CSV")

		if instance.Spec.MeterdefinitionCatalogServerConfig.SyncCommunityMeterDefinitions {
			communityMeterdefs, err := r.catalogClient.ListMeterdefintionsFromFileServer(csvName, csvVersion, CSV.Namespace, reqLogger)
			if err != nil {
				return reconcile.Result{}, err
			}

			result := r.createMeterDefs(communityMeterdefs,CSV, reqLogger)
			if !result.Is(Continue) {
				return result.Return()
			}
		}

		if instance.Spec.MeterdefinitionCatalogServerConfig.SyncSystemMeterDefinitions {
			reqLogger.Info("system meterdefs enabled")
			
			systemMeterDefs, err := r.catalogClient.GetSystemMeterdefs(CSV, reqLogger)
			if err != nil {
				return reconcile.Result{}, err
			}

			result := r.createMeterDefs(systemMeterDefs,CSV, reqLogger)
			if !result.Is(Continue) {
				return result.Return()
			}
		}
	}

	reqLogger.Info("reconciliation complete")
	return reconcile.Result{}, nil
}

func (r *MeterDefinitionInstallReconciler) createMeterDefs(mdefs []marketplacev1beta1.MeterDefinition, CSV *olmv1alpha1.ClusterServiceVersion, reqLogger logr.InfoLogger) *ExecResult {
	csvName := CSV.Name
	csvVersion := CSV.Spec.Version.Version.String()
	
	reqLogger.Info("creating meterdefinitions", "CSV", csvName, "namespace", CSV.Namespace)
	for _, meterDefItem := range mdefs {
		reqLogger.Info("checking for existing meterdefinition", "meterdef", meterDefItem.Name, "CSV", csvName)

		meterdef := &marketplacev1beta1.MeterDefinition{}
		err := r.Client.Get(context.TODO(), types.NamespacedName{Name: meterDefItem.Name, Namespace: CSV.Namespace}, meterdef)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				reqLogger.Info("meterdefinition not found, creating", "meterdef name", meterDefItem.Name, "CSV", CSV.Name, "csv namespace", CSV.Namespace)

				err = r.createMeterdefWithOwnerRef(csvVersion, meterDefItem, CSV, reqLogger)
				if err != nil {
					msg := fmt.Sprintf("error while creating meterdef, meterdef name: %s, csv: %s, csv version: %s", meterDefItem.Name, csvName, csvVersion)
					return &ExecResult{
						ReconcileResult: reconcile.Result{},
						Err:             emperrors.Wrap(err, msg),
					}
				}

				reqLogger.Info("created meterdefinition", "meterdef name", meterDefItem.Name, "CSV", CSV.Name)

				return &ExecResult{
					ReconcileResult: reconcile.Result{Requeue: true},
					Err:             nil,
				}
			}

			reqLogger.Error(err, "Failed to get meterdefinition", "meterdef name", meterDefItem.Name, "CSV", CSV.Name)
			return &ExecResult{
				ReconcileResult: reconcile.Result{},
				Err:             err,
			}
		}
	}

	return &ExecResult{
		Status: ActionResultStatus(Continue),
	}
}

// TODO: not using this right now - relying on the olm.CopiedFrom predicate filter to noop on copies from olm
func (r *MeterDefinitionInstallReconciler) getOperatorGroupNamespace (ogNameFromCSV string) (string,error) {

		ogList := &olmv1.OperatorGroupList{}
		// listOpts := []client.ListOption{}
		err := r.Client.List(context.TODO(),ogList)
		if err != nil {
			return "",err
		}

		var foundOperatorGroup *olmv1.OperatorGroup
		var found bool
		for _, og := range ogList.Items {
			if og.Name == ogNameFromCSV {
				foundOperatorGroup = &og
				found = true
			}
		}

		if found {
			return foundOperatorGroup.Namespace,nil
		}

		return "",errors.New("could not find operator group")
}

func (r *MeterDefinitionInstallReconciler) createMeterdefWithOwnerRef(csvVersion string, meterDefinition marketplacev1beta1.MeterDefinition, csv *olmv1alpha1.ClusterServiceVersion, reqLogger logr.InfoLogger) error {
	groupVersionKind, err := apiutil.GVKForObject(csv, r.Scheme)
	if err != nil {
		return err
	}

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

	err = r.Client.Create(context.TODO(), &meterDefinition)
	if err != nil {
		reqLogger.Error(err, "Could not create meterdefinition", "mdef", meterDefName, "CSV", csv.Name)
		return err
	}

	reqLogger.Info("Created meterdefinition", "mdef", meterDefName, "CSV", csv.Name)

	return nil
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

func isCopy(metaNew metav1.Object) bool {
	labels := metaNew.GetLabels()
	_, labelHasCopiedFromTag := labels[olmCopiedFromTag]

	ann := metaNew.GetAnnotations()
	_, annHasCopiedFromTag := ann[olmCopiedFromTag]

	if !labelHasCopiedFromTag && !annHasCopiedFromTag {
		// is not a copy from an AllNamespaces install
		return false
	}

	// either labels or annotations has the olm.CopiedFrom tag
	return true
}

var rhmCSVControllerPredicates predicate.Funcs = predicate.Funcs{
	UpdateFunc: func(e event.UpdateEvent) bool {
		return !isCopy(e.MetaNew) && checkForCSVVersionChanges(e)
	},

	DeleteFunc: func(e event.DeleteEvent) bool {
		return !isCopy(e.Meta)
	},

	CreateFunc: func(e event.CreateEvent) bool {
		return !isCopy(e.Meta)

	},

	GenericFunc: func(e event.GenericEvent) bool {
		return true
	},
}

func (r *MeterDefinitionInstallReconciler) Inject(injector mktypes.Injectable) mktypes.SetupWithManager {
	injector.SetCustomFields(r)
	return r
}

func (m *MeterDefinitionInstallReconciler) InjectOperatorConfig(cfg *config.OperatorConfig) error {
	m.cfg = cfg
	return nil
}

func (r *MeterDefinitionInstallReconciler) InjectCatalogClient(catalogClient *catalog.CatalogClient) error {
	r.Log.Info("catalog client")
	r.catalogClient = catalogClient
	return nil
}

func (r *MeterDefinitionInstallReconciler) SetupWithManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&olmv1alpha1.ClusterServiceVersion{}, builder.WithPredicates(rhmCSVControllerPredicates)).
		Complete(r)
}
