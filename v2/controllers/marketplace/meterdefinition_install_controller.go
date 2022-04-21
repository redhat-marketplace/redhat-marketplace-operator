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
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"

	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	marketplacev1beta1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/catalog"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"

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
	CatalogClient *catalog.CatalogClient
}

func hasOperatorTag(meta metav1.Object) bool {
	if value, ok := meta.GetLabels()[utils.OperatorTag]; ok {
		if value == utils.OperatorTagValue {
			return true
		}
	}

	return false
}

var rhmSubPredicates predicate.Funcs = predicate.Funcs{
	UpdateFunc: func(e event.UpdateEvent) bool {
		return hasOperatorTag(e.ObjectNew)
	},

	DeleteFunc: func(e event.DeleteEvent) bool {
		return hasOperatorTag(e.Object)
	},

	CreateFunc: func(e event.CreateEvent) bool {
		return hasOperatorTag(e.Object)

	},

	GenericFunc: func(e event.GenericEvent) bool {
		return hasOperatorTag(e.Object)
	},
}

func (r *MeterDefinitionInstallReconciler) Inject(injector mktypes.Injectable) mktypes.SetupWithManager {
	injector.SetCustomFields(r)
	return r
}

func (r *MeterDefinitionInstallReconciler) InjectOperatorConfig(cfg *config.OperatorConfig) error {
	r.cfg = cfg
	return nil
}

func (r *MeterDefinitionInstallReconciler) InjectCatalogClient(catalogClient *catalog.CatalogClient) error {
	r.Log.Info("catalog client")
	r.CatalogClient = catalogClient
	return nil
}

func (r *MeterDefinitionInstallReconciler) SetupWithManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&olmv1alpha1.Subscription{}, builder.WithPredicates(rhmSubPredicates)).
		Complete(r)
}

// +kubebuilder:rbac:groups="operators.coreos.com",resources=clusterserviceversions;subscriptions,verbs=get;list;watch
// +kubebuilder:rbac:groups="operators.coreos.com",resources=clusterserviceversions,verbs=update;patch
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=meterdefinitions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=meterdefinitions;meterdefinitions/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=marketplace.redhat.com,resources=meterdefinitions;meterdefinitions/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="operators.coreos.com",resources=clusterserviceversions;subscriptions,verbs=get;list;watch
// +kubebuilder:rbac:urls=/get-community-meterdefs,verbs=get;post;create;
// +kubebuilder:rbac:urls=/get-system-meterdefs/*,verbs=get;post;create;
// +kubebuilder:rbac:groups="authentication.k8s.io",resources=tokenreviews,verbs=create;get
// +kubebuilder:rbac:groups="authorization.k8s.io",resources=subjectaccessreviews,verbs=create;get

// Reconcile reads that state of the cluster for a ClusterServiceVersion object and creates corresponding meter definitions if found
func (r *MeterDefinitionInstallReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Name", request.Name, "Request.Namespace", request.Namespace)
	reqLogger.Info("Reconciling Object")

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

	// Fetch the subscription instance
	sub := &olmv1alpha1.Subscription{}
	err = r.Client.Get(context.TODO(), request.NamespacedName, sub)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Request object not found, check the meterdef store if there is an existing InstallMapping,delete, and return empty result
			reqLogger.Info("subscription does not exist", "name", request.Name)
			return reconcile.Result{}, nil
		}

		reqLogger.Error(err, "Failed to get subscription")
		return reconcile.Result{}, err
	}

	if sub.Status.InstalledCSV == "" {
		reqLogger.Info("InstalledCSV not set on subscription")
		// stop reconciling, we'll get the next update event when InstalledCSV is set
		return reconcile.Result{}, nil
	}

	csvName := sub.Status.InstalledCSV
	reqLogger.Info("Subscription has InstalledCSV", "csv", csvName)

	// try to find InstalledCSV in the same namespace as the subscription
	csv := &olmv1alpha1.ClusterServiceVersion{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: csvName, Namespace: sub.Namespace}, csv)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			reqLogger.Info("no csv found for InstalledCSV", "csv", csvName, "sub", sub.Name, "sub namespace", sub.Namespace)
			return reconcile.Result{}, nil
		}

		reqLogger.Error(err, "Failed to get clusterserviceversion")
		return reconcile.Result{}, err
	}

	reqLogger.Info("found csv from subscription", "csv", csv.Name, "csv namespace", csv.Namespace)

	cr := &catalog.CatalogRequest{
		CSVInfo: catalog.CSVInfo{
			Name:      csvName,
			Namespace: csv.Namespace,
			Version:   fmt.Sprintf("%d.%d.%d", csv.Spec.Version.Version.Major, csv.Spec.Version.Version.Minor, csv.Spec.Version.Version.Patch),
		},
		SubInfo: catalog.SubInfo{
			PackageName:   sub.Spec.Package,
			CatalogSource: sub.Spec.CatalogSource,
		},
	}

	if instance.Spec.MeterdefinitionCatalogServerConfig != nil {
		if instance.Spec.MeterdefinitionCatalogServerConfig.SyncCommunityMeterDefinitions {
			communityMeterdefs, err := r.CatalogClient.ListMeterdefintionsFromFileServer(cr)
			if err != nil {
				reqLogger.Error(err, "error getting community meterdefs()")
			}

			for _, m := range communityMeterdefs {
				reqLogger.Info("community meterdef returned from file server", "name", m.Name)
			}

			if err == nil {
				err = r.createOrUpdateMeterDefs(communityMeterdefs, csv, reqLogger)
				if err != nil {
					reqLogger.Error(err, "error creating meterdefs")
				}
			}
		}
	}

	if instance.Spec.MeterdefinitionCatalogServerConfig != nil {
		if instance.Spec.MeterdefinitionCatalogServerConfig.SyncSystemMeterDefinitions {
			reqLogger.Info("system meterdefs enabled")
			systemMeterDefs, err := r.CatalogClient.GetSystemMeterdefs(csv, sub.Spec.Package, sub.Spec.CatalogSource)
			if err != nil {
				reqLogger.Error(err, "error getting system meterdefs")
			}

			for _, m := range systemMeterDefs {
				reqLogger.Info("system meterdef returned from file server", "name", m.Name)
			}

			if err == nil {
				err = r.createOrUpdateMeterDefs(systemMeterDefs, csv, reqLogger)
				if err != nil {
					reqLogger.Error(err, "error creating meterdefs")
				}
			}
		}
	}

	reqLogger.Info("reconciliation complete")
	return reconcile.Result{}, nil
}

// TODO: handle updates
func (r *MeterDefinitionInstallReconciler) createOrUpdateMeterDefs(catalogMeterDefs []marketplacev1beta1.MeterDefinition, csv *olmv1alpha1.ClusterServiceVersion, reqLogger logr.Logger) error {
	csvName := csv.Name
	csvVersion := csv.Spec.Version.Version.String()

	reqLogger.Info("creating meterdefinitions for csv", "csv", csvName, "namespace", csv.Namespace)

	for _, catalogMeterDef := range catalogMeterDefs {
		errorFromRetry := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			clusterMeterdef := &marketplacev1beta1.MeterDefinition{}
			err := r.Client.Get(context.TODO(), types.NamespacedName{Name: catalogMeterDef.Name, Namespace: csv.Namespace}, clusterMeterdef)
			if err != nil {
				if k8serrors.IsNotFound(err) {
					reqLogger.Info("meterdefinition not found, creating", "meterdef", catalogMeterDef.Name)

					err = r.createMeterdefWithOwnerRef(csvVersion, &catalogMeterDef, csv)
					if err != nil {
						return fmt.Errorf("error while creating meterdef: %w, meterdef name: %s", err, catalogMeterDef.Name)
					}

					reqLogger.Info("CREATE: created meterdefinition", "meterdef name", catalogMeterDef.Name, "csv", csv.Name)
					// exit retry
					return nil

				}

				return err
			}

			reqLogger.Info("meterdefinition already present, checking for updates", "meterdef", clusterMeterdef.Name)
			err = r.updateMeterdef(clusterMeterdef, catalogMeterDef, reqLogger)
			if err != nil {
				return err
			}

			// exit retry
			return nil
		})

		if errorFromRetry != nil {
			return errorFromRetry
		}
	}

	return nil
}

func (r *MeterDefinitionInstallReconciler) updateMeterdef(onClusterMeterDef *marketplacev1beta1.MeterDefinition, catalogMdef marketplacev1beta1.MeterDefinition, reqLogger logr.Logger) error {
	updatedMeterdefinition := onClusterMeterDef.DeepCopy()
	updatedMeterdefinition.Spec = catalogMdef.Spec
	updatedMeterdefinition.ObjectMeta.Annotations = catalogMdef.ObjectMeta.Annotations

	if !reflect.DeepEqual(updatedMeterdefinition, onClusterMeterDef) {
		reqLogger.Info("meterdefintion is out of sync with latest meterdef catalog", "name", onClusterMeterDef.Name)
		err := r.Client.Update(context.TODO(), updatedMeterdefinition)
		if err != nil {
			reqLogger.Error(err, "Failed updating meter definition", "name", updatedMeterdefinition.Name, "namespace", updatedMeterdefinition.Namespace)
			return err
		}

		reqLogger.Info("UPDATE: updated meterdefintion", "name", updatedMeterdefinition.Name, "namespace", updatedMeterdefinition.Namespace)
		return nil
	}

	return nil
}

func (r *MeterDefinitionInstallReconciler) createMeterdefWithOwnerRef(csvVersion string, meterDefinition *marketplacev1beta1.MeterDefinition, csv *olmv1alpha1.ClusterServiceVersion) error {
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

	err = r.Client.Create(context.TODO(), meterDefinition)
	if err != nil {
		return err
	}

	return nil
}
