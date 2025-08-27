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

	emperrors "emperror.dev/errors"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/config"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/operrors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/predicates"
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

	Cfg     *config.OperatorConfig
	Factory *manifests.Factory
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func (r *DataServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	namespacePredicate := predicates.NamespacePredicate(r.Cfg.DeployedNamespace)

	return ctrl.NewControllerManagedBy(mgr).
		Named("data-service").
		For(&marketplacev1alpha1.MeterBase{}).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &marketplacev1alpha1.MeterBase{}, handler.OnlyControllerOwner()),
			builder.WithPredicates(namespacePredicate)).
		Watches(
			&corev1.Service{},
			handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &marketplacev1alpha1.MeterBase{}, handler.OnlyControllerOwner()),
			builder.WithPredicates(namespacePredicate)).
		Watches(
			&appsv1.StatefulSet{},
			handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &marketplacev1alpha1.MeterBase{}, handler.OnlyControllerOwner()),
			builder.WithPredicates(namespacePredicate)).
		Watches(
			&routev1.Route{},
			handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &marketplacev1alpha1.MeterBase{}, handler.OnlyControllerOwner()),
			builder.WithPredicates(namespacePredicate)).Complete(r)
}

// +kubebuilder:rbac:groups="",namespace=system,resources=secrets,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="",namespace=system,resourceNames=rhm-data-service-mtls,resources=secrets,verbs=patch;update;delete
// +kubebuilder:rbac:groups="",namespace=system,resources=services,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="",namespace=system,resourceNames=rhm-data-service,resources=services,verbs=patch;update;delete
// +kubebuilder:rbac:groups="apps",namespace=system,resources=statefulsets,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="apps",namespace=system,resources=statefulsets,verbs=patch;update;delete,resourceNames=rhm-data-service
// +kubebuilder:rbac:groups="route.openshift.io",namespace=system,resources=routes,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="route.openshift.io",namespace=system,resources=routes,verbs=patch;update;delete,resourceNames=rhm-data-service

// Reconcile reads that state of the cluster for a MeterBase object and makes changes based on the state read
// and what is in the MeterBase.Spec
func (r *DataServiceReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling DataService")

	// Fetch the MeterBase instance
	meterBase := &marketplacev1alpha1.MeterBase{}
	if err := r.Client.Get(context.TODO(), request.NamespacedName, meterBase); err != nil {
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
		/* DataService mTLS certificate Secret
		Generate custom secret instead of using service.beta.openshift.io/serving-cert-secret-name
		The CommonName needs a *.prefix for pod-to-pod communication
		*/
		secret, err := r.Factory.NewDataServiceTLSSecret(utils.DQLITE_COMMONNAME_PREFIX)
		if err != nil {
			reqLogger.Error(err, "Generate Secret error: ")
			return reconcile.Result{}, err
		}
		if err := r.Factory.SetControllerReference(meterBase, secret); err != nil {
			return reconcile.Result{}, err
		}

		foundSecret := &corev1.Secret{}
		err = r.Client.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, foundSecret)
		if err != nil && errors.IsNotFound(err) { // not found: create
			err = r.Client.Create(ctx, secret)
			if err != nil {
				reqLogger.Error(err, "Create Secret error: ")
				return reconcile.Result{}, err
			}
		} else if err != nil {
			reqLogger.Error(err, "Get Secret error: ")
			return reconcile.Result{}, err
		}

		/* DataService Service */
		if err := r.Factory.CreateOrUpdate(r.Client, meterBase, func() (client.Object, error) {
			return r.Factory.NewDataServiceService()
		}); err != nil {
			return reconcile.Result{}, err
		}

		/* DataService StatefulSet */
		storageClassName, err := r.getStorageClassName(ctx, meterBase)
		if err != nil {
			return reconcile.Result{}, err
		}
		if err := r.Factory.CreateOrUpdate(r.Client, meterBase, func() (client.Object, error) {
			return r.Factory.NewDataServiceStatefulSet(storageClassName)
		}); err != nil {
			return reconcile.Result{}, err
		}

		/* DataService Route */
		if err := r.Factory.CreateOrUpdate(r.Client, meterBase, func() (client.Object, error) {
			return r.Factory.NewDataServiceRoute()
		}); err != nil {
			return reconcile.Result{}, err
		}

	} else { // Remove the DataService
		/* DataService Route*/
		route, err := r.Factory.NewDataServiceRoute()
		if err != nil {
			reqLogger.Error(err, "data service route error")
			return reconcile.Result{}, err
		}

		if err := r.Client.Delete(ctx, route); err != nil && !errors.IsNotFound(err) {
			reqLogger.Error(err, "Delete Route error: ")
			return reconcile.Result{}, err
		}

		/* DataService StatefulSet*/
		statefulSet, err := r.Factory.NewDataServiceStatefulSet(nil)
		if err != nil {
			reqLogger.Error(err, "data service statefulset error")
			return reconcile.Result{}, err
		}

		if err := r.Client.Delete(ctx, statefulSet); err != nil && !errors.IsNotFound(err) {
			reqLogger.Error(err, "Delete StatefulSet error: ")
			return reconcile.Result{}, err
		}

		/* DataService Service */
		service, err := r.Factory.NewDataServiceService()
		if err != nil {
			reqLogger.Error(err, "data service route error")
			return reconcile.Result{}, err
		}

		if err := r.Client.Delete(ctx, service); err != nil && !errors.IsNotFound(err) {
			reqLogger.Error(err, "Delete Service error: ")
			return reconcile.Result{}, err
		}

		/* DataService Secret */
		secret, err := r.Factory.NewDataServiceTLSSecret(utils.DQLITE_COMMONNAME_PREFIX)
		if err != nil {
			reqLogger.Error(err, "data service secret error")
			return reconcile.Result{}, err
		}

		if err = r.Client.Delete(ctx, secret); err != nil && !errors.IsNotFound(err) {
			reqLogger.Error(err, "Delete Secret error: ")
			return reconcile.Result{}, err
		}
	}
	reqLogger.Info("finished reconciling")
	return reconcile.Result{}, nil
}

// Find the appropriate storageClassName to use for the StatefulSet VolumeClaimTemplate
// Check Spec
// Check existing PVC
// Check defaultStorgeClass
func (r *DataServiceReconciler) getStorageClassName(ctx context.Context, meterBase *marketplacev1alpha1.MeterBase) (*string, error) {
	// StorageClassName from MeterBase Spec
	if meterBase.Spec.StorageClassName != nil {
		return meterBase.Spec.StorageClassName, nil
	}
	// StorageClassName from pre-created PersistentVolumeClaim rhm-data-service-rhm-data-service-0
	persistentVolumeClaim := &corev1.PersistentVolumeClaim{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: "rhm-data-service-rhm-data-service-0", Namespace: meterBase.Namespace}, persistentVolumeClaim)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	} else if persistentVolumeClaim.Spec.StorageClassName != nil {
		return persistentVolumeClaim.Spec.StorageClassName, nil
	}
	// StorageClassName from the default StorageClass
	defaultStorageClass, err := utils.GetDefaultStorageClass(r.Client)
	if err != nil {
		switch {
		case emperrors.Is(err, operrors.DefaultStorageClassNotFound):
			return nil, nil
		case emperrors.Is(err, operrors.MultipleDefaultStorageClassFound):
			return nil, nil
		default:
			return nil, err
		}
	} else {
		return &defaultStorageClass, nil
	}
}
