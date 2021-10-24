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

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/patch"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/predicates"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	ctrl "sigs.k8s.io/controller-runtime"

	routev1 "github.com/openshift/api/route/v1"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// blank assignment to verify that ReconcileDataService implements reconcile.Reconciler
var _ reconcile.Reconciler = &DataServiceReconciler{}

// DataServiceReconciler reconciles the DataService of a MeterBase object
type DataServiceReconciler struct {
	// This Client, initialized using mgr.Client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	Client client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
	CC     ClientCommandRunner

	cfg     *config.OperatorConfig
	factory *manifests.Factory
	patcher patch.Patcher
}

func (r *DataServiceReconciler) Inject(injector mktypes.Injectable) mktypes.SetupWithManager {
	injector.SetCustomFields(r)
	return r
}

func (r *DataServiceReconciler) InjectOperatorConfig(cfg *config.OperatorConfig) error {
	r.cfg = cfg
	return nil
}

func (r *DataServiceReconciler) InjectCommandRunner(ccp ClientCommandRunner) error {
	r.Log.Info("command runner")
	r.CC = ccp
	return nil
}

func (r *DataServiceReconciler) InjectPatch(p patch.Patcher) error {
	r.patcher = p
	return nil
}

func (r *DataServiceReconciler) InjectFactory(f *manifests.Factory) error {
	r.factory = f
	return nil
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func (r *DataServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {

	namespacePredicate := predicates.NamespacePredicate(r.cfg.DeployedNamespace)

	return ctrl.NewControllerManagedBy(mgr).
		For(&marketplacev1alpha1.MeterBase{}).
		Watches(
			&source.Kind{Type: &corev1.Secret{}},
			&handler.EnqueueRequestForOwner{
				IsController: true,
				OwnerType:    &marketplacev1alpha1.MeterBase{}},
			builder.WithPredicates(namespacePredicate)).
		Watches(
			&source.Kind{Type: &corev1.Service{}},
			&handler.EnqueueRequestForOwner{
				IsController: true,
				OwnerType:    &marketplacev1alpha1.MeterBase{}},
			builder.WithPredicates(namespacePredicate)).
		Watches(
			&source.Kind{Type: &appsv1.StatefulSet{}},
			&handler.EnqueueRequestForOwner{
				IsController: true,
				OwnerType:    &marketplacev1alpha1.MeterBase{}},
			builder.WithPredicates(namespacePredicate)).
		Watches(
			&source.Kind{Type: &routev1.Route{}},
			&handler.EnqueueRequestForOwner{
				IsController: true,
				OwnerType:    &marketplacev1alpha1.MeterBase{}},
			builder.WithPredicates(namespacePredicate)).Complete(r)
}

// +kubebuilder:rbac:groups="",namespace=system,resources=services,verbs=get;list;watch;create;patch;update;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups="apps",namespace=system,resources=statefulsets,verbs=get;list;watch;create;patch;update;delete
// +kubebuilder:rbac:groups="apps",resources=statefulsets,verbs=get;list;watch
// +kubebuilder:rbac:groups="route.openshift.io",namespace=system,resources=routes,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="route.openshift.io",namespace=system,resources=routes,verbs=get;patch;update;delete,resourceNames=rhm-data-service
// +kubebuilder:rbac:groups="route.openshift.io",resources=routes,verbs=get;list;watch
// +kubebuilder:rbac:urls=*,verbs=create

// Reconcile reads that state of the cluster for a MeterBase object and makes changes based on the state read
// and what is in the MeterBase.Spec
func (r *DataServiceReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling DataService")

	// Fetch the MeterBase instance
	meterBase := &marketplacev1alpha1.MeterBase{}
	err := r.Client.Get(context.TODO(), request.NamespacedName, meterBase)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("Resource not found. Ignoring since object must be deleted")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Failed to get MeterBase")
		return reconcile.Result{}, err
	}

	if meterBase.Spec.IsDataServiceEnabled() { // Install the DataService
		/* DataService mTLS certificate Secret */
		secret, err := r.factory.NewDataServiceTLSSecret(utils.DQLITE_COMMONNAME_PREFIX)
		if err != nil {
			reqLogger.Error(err, "Generate Secret error: ")
			return reconcile.Result{}, err
		}
		r.factory.SetControllerReference(meterBase, secret)
		foundSecret := &corev1.Secret{}
		err = r.Client.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, foundSecret)
		if err != nil && errors.IsNotFound(err) { // not found: create & requeue
			err = r.Client.Create(ctx, secret)
			if err != nil {
				reqLogger.Error(err, "Create Secret error: ")
				return reconcile.Result{}, err
			}
			return reconcile.Result{Requeue: true}, nil
		} else if err != nil {
			reqLogger.Error(err, "Get Secret error: ")
			return reconcile.Result{}, err
		}

		/* DataService Service */
		service, err := r.factory.NewDataServiceService()

		if err != nil {
			reqLogger.Error(err, "data service service error")
			return reconcile.Result{}, err
		}

		r.factory.SetControllerReference(meterBase, service)

		foundService := &corev1.Service{}
		err = r.Client.Get(ctx, types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, foundService)
		if err != nil && errors.IsNotFound(err) { // not found: create & requeue
			err = r.Client.Create(ctx, service)
			if err != nil {
				reqLogger.Error(err, "Create Service error: ")
				return reconcile.Result{}, err
			}
			return reconcile.Result{Requeue: true}, nil
		} else if err != nil {
			reqLogger.Error(err, "Get Service error: ")
			return reconcile.Result{}, err
		} else { // found: enforce spec
			r.factory.UpdateDataServiceService(foundService)
			err = r.Client.Update(ctx, foundService)
			if err != nil {
				reqLogger.Error(err, "Patch Service error: ")
				return reconcile.Result{}, err
			}
		}
		/* DataService StatefulSet */
		statefulSet, err := r.factory.NewDataServiceStatefulSet()

		if err != nil {
			reqLogger.Error(err, "data service statefulset error")
			return reconcile.Result{}, err
		}

		r.factory.SetControllerReference(meterBase, statefulSet)
		foundStatefulSet := &appsv1.StatefulSet{}
		err = r.Client.Get(ctx, types.NamespacedName{Name: statefulSet.Name, Namespace: statefulSet.Namespace}, foundStatefulSet)
		if err != nil && errors.IsNotFound(err) { // not found: create & requeue
			err = r.Client.Create(ctx, statefulSet)
			if err != nil {
				reqLogger.Error(err, "Create StatefulSet error: ")
				return reconcile.Result{}, err
			}
			return reconcile.Result{Requeue: true}, nil
		} else if err != nil {
			reqLogger.Error(err, "Get StatefulSet error: ")
			return reconcile.Result{}, err
		} else { // found: enforce spec
			r.factory.UpdateDataServiceStatefulSet(foundStatefulSet)
			err = r.Client.Update(ctx, foundStatefulSet)
			if err != nil {
				reqLogger.Error(err, "Update StatefulSet error: ")
				return reconcile.Result{}, err
			}
		}
		/* DataService Route */
		route, err := r.factory.NewDataServiceRoute()

		if err != nil {
			reqLogger.Error(err, "data service route error")
			return reconcile.Result{}, err
		}

		r.factory.SetControllerReference(meterBase, route)
		foundRoute := &routev1.Route{}
		err = r.Client.Get(ctx, types.NamespacedName{Name: route.Name, Namespace: route.Namespace}, foundRoute)
		if err != nil && errors.IsNotFound(err) { // not found: create & requeue
			err = r.Client.Create(ctx, route)
			if err != nil {
				reqLogger.Error(err, "Create Route error: ")
				return reconcile.Result{}, err
			}
			return reconcile.Result{Requeue: true}, nil
		} else if err != nil {
			reqLogger.Error(err, "Get Route error: ")
			return reconcile.Result{}, err
		} else { // found: enforce spec
			r.factory.UpdateDataServiceRoute(foundRoute)
			err = r.Client.Update(ctx, foundRoute)
			if err != nil {
				reqLogger.Error(err, "Update Route error: ")
				return reconcile.Result{}, err
			}
		}
	} else { // Remove the DataService
		/* DataService Route*/
		route, err := r.factory.NewDataServiceRoute()

		if err != nil {
			reqLogger.Error(err, "data service route error")
			return reconcile.Result{}, err
		}

		err = r.Client.Delete(ctx, route)
		if err != nil && !errors.IsNotFound(err) {
			reqLogger.Error(err, "Delete Route error: ")
			return reconcile.Result{}, err
		}
		/* DataService StatefulSet*/
		statefulSet, err := r.factory.NewDataServiceStatefulSet()

		if err != nil {
			reqLogger.Error(err, "data service route error")
			return reconcile.Result{}, err
		}

		err = r.Client.Delete(ctx, statefulSet)
		if err != nil && !errors.IsNotFound(err) {
			reqLogger.Error(err, "Delete StatefulSet error: ")
			return reconcile.Result{}, err
		}
		/* DataService Service */
		service, err := r.factory.NewDataServiceService()

		if err != nil {
			reqLogger.Error(err, "data service route error")
			return reconcile.Result{}, err
		}

		err = r.Client.Delete(ctx, service)
		if err != nil && !errors.IsNotFound(err) {
			reqLogger.Error(err, "Delete Service error: ")
			return reconcile.Result{}, err
		}
		/* DataService Secret */
		secret, err := r.factory.NewDataServiceTLSSecret(utils.DQLITE_COMMONNAME_PREFIX)
		err = r.Client.Delete(ctx, secret)
		if err != nil && !errors.IsNotFound(err) {
			reqLogger.Error(err, "Delete Secret error: ")
			return reconcile.Result{}, err
		}
	}
	reqLogger.Info("finished reconciling")
	return reconcile.Result{}, nil
}
