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
	// "bytes"

	"time"

	"github.com/go-logr/logr"

	osappsv1 "github.com/openshift/api/apps/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/patch"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/predicates"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"

	// k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"

	osimagev1 "github.com/openshift/api/image/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/catalog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// blank assignment to verify that DeploymentConfigReconciler implements reconcile.Reconciler
var _ reconcile.Reconciler = &DeploymentConfigReconciler{}

// var GlobalMeterdefStoreDB = &MeterdefStoreDB{}
// DeploymentConfigReconciler reconciles the DataService of a MeterBase object
type SubSectionReconciler struct {
	// This Client, initialized using mgr.Client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	Client client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
	CC     ClientCommandRunner
	kubeInterface kubernetes.Interface
	cfg           *config.OperatorConfig
	factory       *manifests.Factory
	patcher       patch.Patcher
	CatalogClient *catalog.CatalogClient
}

func (r *SubSectionReconciler) Inject(injector mktypes.Injectable) mktypes.SetupWithManager {
	injector.SetCustomFields(r)
	return r
}

func (r *SubSectionReconciler) InjectOperatorConfig(cfg *config.OperatorConfig) error {
	r.cfg = cfg
	return nil
}

func (r *SubSectionReconciler) InjectCommandRunner(ccp ClientCommandRunner) error {
	r.Log.Info("command runner")
	r.CC = ccp
	return nil
}

func (r *SubSectionReconciler) InjectCatalogClient(catalogClient *catalog.CatalogClient) error {
	r.Log.Info("catalog client")
	r.CatalogClient = catalogClient
	return nil
}

func (r *SubSectionReconciler) InjectPatch(p patch.Patcher) error {
	r.patcher = p
	return nil
}

func (r *SubSectionReconciler) InjectFactory(f *manifests.Factory) error {
	r.factory = f
	return nil
}

func (r *SubSectionReconciler) InjectKubeInterface(k kubernetes.Interface) error {
	r.kubeInterface = k
	return nil
}

// adds a new Controller to mgr with r as the reconcile.Reconciler
func (r *SubSectionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Log.Info("SetupWithManager SubSectionReconciler")

	nsPred := predicates.NamespacePredicate(r.cfg.DeployedNamespace)

	// meterBaseSubSectionPred := []predicate.Predicate{
	// 	predicate.Funcs{
	// 		CreateFunc: func(e event.CreateEvent) bool {
	// 			return true
	// 		},
	// 		UpdateFunc: func(e event.UpdateEvent) bool {
	// 			meterbaseOld, ok := e.ObjectOld.(*marketplacev1alpha1.MeterBase)
	// 			if !ok {
	// 				return false
	// 			}

	// 			meterbaseNew, ok := e.ObjectNew.(*marketplacev1alpha1.MeterBase)
	// 			if !ok {
	// 				return false
	// 			}

	// 			return meterbaseOld.Spec.MeterdefinitionCatalogServer != meterbaseNew.Spec.MeterdefinitionCatalogServer
	// 		},
	// 		DeleteFunc: func(e event.DeleteEvent) bool {
	// 			return true
	// 		},
	// 		GenericFunc: func(e event.GenericEvent) bool {
	// 			return true
	// 		},
	// 	},
	// }

	deploymentConfigPred := []predicate.Predicate{
		predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return e.Meta.GetName() == utils.DEPLOYMENT_CONFIG_NAME
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				return e.MetaNew.GetName() == utils.DEPLOYMENT_CONFIG_NAME
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return e.Meta.GetName() == utils.DEPLOYMENT_CONFIG_NAME
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return e.Meta.GetName() == utils.DEPLOYMENT_CONFIG_NAME
			},
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(nsPred).
		For(&osappsv1.DeploymentConfig{},builder.WithPredicates(deploymentConfigPred...)).
		// Watches(
		// 	&source.Kind{Type: &marketplacev1alpha1.MeterBase{}},
		// 	&handler.EnqueueRequestForObject{},
		// 	builder.WithPredicates(meterBaseSubSectionPred...)).
		// Watches(
		// 	&source.Kind{Type: &osappsv1.DeploymentConfig{}},
		// 	&handler.EnqueueRequestForObject{},
		// 	builder.WithPredicates(deploymentConfigPred...)).
		Watches(
			&source.Kind{Type: &osimagev1.ImageStream{}},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(deploymentConfigPred...)).
		Complete(r)
}

// +kubebuilder:rbac:groups=apps.openshift.io,resources=deploymentconfigs,verbs=get;list;update;watch
// +kubebuilder:rbac:groups=image.openshift.io,resources=imagestreams,verbs=get;create;update;list;watch
// +kubebuilder:rbac:urls=/list-for-version/*,verbs=get;
// +kubebuilder:rbac:urls=/get-system-meterdefs/*,verbs=get;post;create;
// +kubebuilder:rbac:urls=/meterdef-index-label/*,verbs=get;
// +kubebuilder:rbac:groups="authentication.k8s.io",resources=tokenreviews,verbs=create;get
// +kubebuilder:rbac:groups="authorization.k8s.io",resources=subjectaccessreviews,verbs=create;get

// Reconcile reads that state of the cluster for a DeploymentConfig object and makes changes based on the state read
// and what is in the MeterdefConfigmap.Spec
func (r *SubSectionReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)

	
	reqLogger.Info("finished reconciling")
	return reconcile.Result{RequeueAfter: time.Minute * 1}, nil
}






