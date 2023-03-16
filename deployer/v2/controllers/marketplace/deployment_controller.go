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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"

	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
	mktypes "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/types"
	utils "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const ()

// blank assignment to verify that ReconcileOperator implements reconcile.Reconciler
var _ reconcile.Reconciler = &DeploymentReconciler{}

// OperatorReconciler reconciles objects essential to the deployment
type DeploymentReconciler struct {
	// This Client, initialized using mgr.Client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	Client  client.Client
	Scheme  *runtime.Scheme
	Log     logr.Logger
	factory *manifests.Factory
}

func (r *DeploymentReconciler) Inject(injector mktypes.Injectable) mktypes.SetupWithManager {
	injector.SetCustomFields(r)
	return r
}

func (r *DeploymentReconciler) InjectFactory(f *manifests.Factory) error {
	r.factory = f
	return nil
}

// +kubebuilder:rbac:groups="apps",namespace=system,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups="",namespace=system,resources=configmaps;secrets;services,verbs=get;list;watch;create;patch;update
// +kubebuilder:rbac:groups="",namespace=system,resources=configmaps;secrets;services,verbs=get;list;watch;create;patch;update
// +kubebuilder:rbac:groups="monitoring.coreos.com",namespace=system,resources=servicemonitors,verbs=get;list;watch;create;patch;update

// Enforce the loose operator bundle manifests. OLM applies these once and does not reconcile their state
// assets/metrics-operator should match the bundle/manifests
func (r *DeploymentReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Name", request.Name, "Request.Namespace", request.Namespace)
	reqLogger.Info("Reconciling Deployment")

	instance := &appsv1.Deployment{}
	err := r.Client.Get(context.TODO(), request.NamespacedName, instance)

	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Error(err, "deployment does not exist")
			return reconcile.Result{}, nil
		}
		reqLogger.Error(err, "Failed to get deployment")
		return reconcile.Result{}, err
	}

	if err := r.factory.CreateOrUpdate(r.Client, nil, func() (client.Object, error) {
		cm, err := r.factory.NewRHMOCABundleConfigMap()
		return cm, err
	}); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.factory.CreateOrUpdate(r.Client, nil, func() (client.Object, error) {
		secret, err := r.factory.NewRHMOServiceMonitorMetricsReaderSecret()
		return secret, err
	}); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.factory.CreateOrUpdate(r.Client, nil, func() (client.Object, error) {
		svc, err := r.factory.NewRHMOMetricsService()
		return svc, err
	}); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.factory.CreateOrUpdate(r.Client, nil, func() (client.Object, error) {
		sm, err := r.factory.NewRHMOMetricsServiceMonitor()
		return sm, err
	}); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *DeploymentReconciler) SetupWithManager(mgr manager.Manager) error {

	// This mapFn will queue deployment/metrics-operator-controller-manager
	mapFn := handler.MapFunc(
		func(obj client.Object) []reconcile.Request {
			return []reconcile.Request{
				{NamespacedName: types.NamespacedName{
					Name:      utils.RHM_METERING_DEPLOYMENT_NAME,
					Namespace: obj.GetNamespace(),
				}},
			}
		})

	pConfigMap := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.ObjectNew.GetName() == utils.RHM_OP_CA_BUNDLE_CONFIGMAP
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return e.Object.GetName() == utils.RHM_OP_CA_BUNDLE_CONFIGMAP
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return e.Object.GetName() == utils.RHM_OP_CA_BUNDLE_CONFIGMAP
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}

	pSecret := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.ObjectNew.GetName() == utils.RHM_OP_METRICS_READER_SECRET
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return e.Object.GetName() == utils.RHM_OP_METRICS_READER_SECRET
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return e.Object.GetName() == utils.RHM_OP_METRICS_READER_SECRET
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}

	pService := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.ObjectNew.GetName() == utils.RHM_OP_METRICS_SERVICE
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return e.Object.GetName() == utils.RHM_OP_METRICS_SERVICE
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return e.Object.GetName() == utils.RHM_OP_METRICS_SERVICE
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}

	pServiceMonitor := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.ObjectNew.GetName() == utils.RHM_OP_SERVICE_MONITOR
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return e.Object.GetName() == utils.RHM_OP_SERVICE_MONITOR
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return e.Object.GetName() == utils.RHM_OP_SERVICE_MONITOR
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}, builder.WithPredicates(clusterServiceVersionPredictates)).
		Watches(&source.Kind{Type: &corev1.ConfigMap{}},
			handler.EnqueueRequestsFromMapFunc(mapFn),
			builder.WithPredicates(pConfigMap)).
		Watches(&source.Kind{Type: &corev1.Secret{}},
			handler.EnqueueRequestsFromMapFunc(mapFn),
			builder.WithPredicates(pSecret)).
		Watches(&source.Kind{Type: &corev1.Service{}},
			handler.EnqueueRequestsFromMapFunc(mapFn),
			builder.WithPredicates(pService)).
		Watches(&source.Kind{Type: &monitoringv1.ServiceMonitor{}},
			handler.EnqueueRequestsFromMapFunc(mapFn),
			builder.WithPredicates(pServiceMonitor)).
		Complete(r)
}
