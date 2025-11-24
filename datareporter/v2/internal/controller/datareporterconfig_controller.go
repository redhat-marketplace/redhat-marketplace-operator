/*
Copyright 2023 IBM Co..

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"

	"dario.cat/mergo"
	"github.com/go-logr/logr"
	"github.com/gotidy/ptr"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/api/v1alpha1"
	datareporterv1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/api/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/datafilter"
	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/pkg/events"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/api/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	status "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// DataReporterConfigReconciler reconciles a DataReporterConfig object
type DataReporterConfigReconciler struct {
	Client        client.Client
	Scheme        *runtime.Scheme
	Log           logr.Logger
	Config        *events.Config
	DataFilters   *datafilter.DataFilters
	secretsSet    watchSet
	configMapsSet watchSet
}

// data-service
//+kubebuilder:rbac:urls=/dataservice.v1.fileserver.FileServer/*,verbs=create
// kube-rbac-proxy
//+kubebuilder:rbac:groups=authentication.k8s.io,resources=tokenreviews,verbs=create
//+kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews,verbs=create

// controller CRDs
//+kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=datareporterconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=datareporterconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=datareporterconfigs/finalizers,verbs=update
//+kubebuilder:rbac:groups=marketplace.redhat.com,namespace=system,resources=marketplaceconfigs,verbs=get;list;watch
//+kubebuilder:rbac:groups=route.openshift.io,namespace=system,resources=routes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",namespace=system,resources=services,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups="",namespace=system,resources=configmaps,verbs=get;list;watch
//+kubebuilder:rbac:groups="",namespace=system,resources=secrets,verbs=get;list;watch

func (r *DataReporterConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)

	// prerequisite marketplaceconfig, otherwise deny events by denying all api keys
	marketplaceConfig := &marketplacev1alpha1.MarketplaceConfig{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: utils.MARKETPLACECONFIG_NAME, Namespace: req.Namespace}, marketplaceConfig); err != nil {
		if k8serrors.IsNotFound(err) {
			reqLogger.Error(err, "marketplaceconfig resource not found. IBM Metrics Operator prerequisite required.")
		}
	} else {
		r.Config.LicenseAccept = ptr.ToBool(marketplaceConfig.Spec.License.Accept)
		if r.Config.LicenseAccept != true {
			reqLogger.Info("license has not been accepted in marketplaceconfig. event handler will not accept events.")

		}
	}

	// if datareporterconfig is not found, create a default so we can set status
	dataReporterConfig := &v1alpha1.DataReporterConfig{}
	if err := r.Client.Get(ctx, req.NamespacedName, dataReporterConfig); err != nil {
		if k8serrors.IsNotFound(err) {
			reqLogger.Info("datareporterconfig resource not found, creating.")
			dataReporterConfig.Name = utils.DATAREPORTERCONFIG_NAME
			dataReporterConfig.Namespace = req.Namespace
			if err := r.Client.Create(ctx, dataReporterConfig); err != nil {
				return ctrl.Result{}, err
			}
		}
		reqLogger.Error(err, "Failed to get datareporterconfig")
		return ctrl.Result{}, err
	}

	reqLogger.Info("datareporterconfig found")

	// Only Watch/Reconcile for Secret/ConfigMap updates that are referenced in the datareporterconfig
	// This prevents unnecessary reconciliation & rebuild of DataFilters, impacting request response
	r.setSecretConfigMapList(dataReporterConfig)

	// check license and update status
	if r.Config.LicenseAccept {
		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			// license accepted, clear status
			if err := r.Client.Get(context.TODO(), req.NamespacedName, dataReporterConfig); err != nil {
				return err
			}
			if dataReporterConfig.Status.Conditions.RemoveCondition(status.ConditionType(datareporterv1alpha1.ConditionNoLicense)) {
				reqLogger.Info("updating dataReporterConfig status")
				return r.Client.Status().Update(context.TODO(), dataReporterConfig)
			}
			return nil
		}); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			// license not accepted, update status
			if err := r.Client.Get(context.TODO(), req.NamespacedName, dataReporterConfig); err != nil {
				return err
			}
			if dataReporterConfig.Status.Conditions.SetCondition(status.Condition{
				Type:    datareporterv1alpha1.ConditionNoLicense,
				Status:  corev1.ConditionTrue,
				Reason:  datareporterv1alpha1.ReasonLicenseNotAccepted,
				Message: "License has not been accepted in marketplaceconfig",
			}) {
				reqLogger.Info("updating dataReporterConfig status")
				return r.Client.Status().Update(context.TODO(), dataReporterConfig)
			}
			return nil
		}); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Set the updated UserConfig for the Event Processor
	r.Config.UserConfigs.SetUserConfigs(dataReporterConfig.Spec.UserConfigs)

	// Enforce the Service Spec for ibm-data-reporter-operator-controller-manager-metrics-service
	// Should be created by OLM at install time, but OLM does not reconcile it when modified/deleted
	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.DATAREPORTER_SERVICE_NAME,
			Namespace: dataReporterConfig.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "https",
					Port:       8443,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromString("https"),
				},
			},
			Selector: map[string]string{
				"control-plane":               "controller-manager",
				"redhat.marketplace.com/name": "ibm-data-reporter-operator",
			},
		},
	}

	// Create the Service
	newService := service
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		_, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, &newService, func() error {
			return mergo.Merge(&newService, &service, mergo.WithOverride)
		})
		return err
	}); err != nil {
		return ctrl.Result{}, err
	}

	// Configure the Route
	route := routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ibm-data-reporter",
			Namespace: dataReporterConfig.Namespace,
		},
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: utils.DATAREPORTER_SERVICE_NAME,
			},
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromInt(8443),
			},
			TLS: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationReencrypt,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			},
		},
	}

	// Create the Route
	newRoute := route
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		_, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, &newRoute, func() error {
			controllerutil.SetControllerReference(dataReporterConfig, &newRoute, r.Scheme)
			return mergo.Merge(&newRoute, &route, mergo.WithOverride)
		})
		return err
	}); err != nil {
		return ctrl.Result{}, err
	}

	r.Log.Info("building datafilters")
	// Build DataFilters
	if derr := r.DataFilters.Build(dataReporterConfig); derr != nil {
		r.Log.Error(derr, "failed to build datafilters")
		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			// DataFilters failed validation, update status
			if err := r.Client.Get(context.TODO(), req.NamespacedName, dataReporterConfig); err != nil {
				return err
			}
			if dataReporterConfig.Status.Conditions.SetCondition(status.Condition{
				Type:    datareporterv1alpha1.ConditionDataFilterInvalid,
				Status:  corev1.ConditionTrue,
				Reason:  datareporterv1alpha1.ReasonDataFilterInvalid,
				Message: derr.Error(),
			}) {
				return r.Client.Status().Update(context.TODO(), dataReporterConfig)
			}
			return nil
		}); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			// DataFilters valid, clear status
			if err := r.Client.Get(context.TODO(), req.NamespacedName, dataReporterConfig); err != nil {
				return err
			}
			if dataReporterConfig.Status.Conditions.RemoveCondition(status.ConditionType(datareporterv1alpha1.ReasonDataFilterInvalid)) {
				return r.Client.Status().Update(context.TODO(), dataReporterConfig)
			}
			return nil
		}); err != nil {
			return ctrl.Result{}, err
		}
	}

	reqLogger.Info("reconcile complete")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DataReporterConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {

	drcPred := predicate.Funcs{
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.ObjectNew.GetName() == utils.DATAREPORTERCONFIG_NAME
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return e.Object.GetName() == utils.DATAREPORTERCONFIG_NAME
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return e.Object.GetName() == utils.DATAREPORTERCONFIG_NAME
		},
	}

	mpcPred := predicate.Funcs{
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.ObjectNew.GetName() == utils.MARKETPLACECONFIG_NAME
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return e.Object.GetName() == utils.MARKETPLACECONFIG_NAME
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return e.Object.GetName() == utils.MARKETPLACECONFIG_NAME
		},
	}

	svcPred := predicate.Funcs{
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.ObjectNew.GetName() == utils.DATAREPORTER_SERVICE_NAME
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return e.Object.GetName() == utils.DATAREPORTER_SERVICE_NAME
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return e.Object.GetName() == utils.DATAREPORTER_SERVICE_NAME
		},
	}

	cmPred := predicate.Funcs{
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {

			return r.configMapsSet.Exists(e.ObjectNew.GetName())
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return r.configMapsSet.Exists(e.Object.GetName())
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return r.configMapsSet.Exists(e.Object.GetName())
		},
	}

	secretPred := predicate.Funcs{
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {

			return r.secretsSet.Exists(e.ObjectNew.GetName())
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return r.secretsSet.Exists(e.Object.GetName())
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return r.secretsSet.Exists(e.Object.GetName())
		},
	}

	mapFn := func(ctx context.Context, a client.Object) []reconcile.Request {
		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      utils.DATAREPORTERCONFIG_NAME,
					Namespace: a.GetNamespace(),
				},
			},
		}
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("datareporterconfig").
		For(&v1alpha1.DataReporterConfig{},
			builder.WithPredicates(drcPred)).
		Watches(
			&marketplacev1alpha1.MarketplaceConfig{},
			handler.EnqueueRequestsFromMapFunc(mapFn),
			builder.WithPredicates(mpcPred)).
		Watches(
			&corev1.Service{},
			handler.EnqueueRequestsFromMapFunc(mapFn),
			builder.WithPredicates(svcPred)).
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(mapFn),
			builder.WithPredicates(cmPred)).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(mapFn),
			builder.WithPredicates(secretPred)).
		Watches(
			&routev1.Route{},
			handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &v1alpha1.DataReporterConfig{}, handler.OnlyControllerOwner()),
		).Complete(r)
}

func generateKey() string {
	b := make([]byte, 32)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func (r *DataReporterConfigReconciler) setSecretConfigMapList(drc *v1alpha1.DataReporterConfig) {
	secretSet := make(map[string]struct{})
	configMapSet := make(map[string]struct{})
	empty := struct{}{}

	if drc.Spec.TLSConfig != nil {
		for _, cacert := range drc.Spec.TLSConfig.CACerts {
			secretSet[cacert.LocalObjectReference.Name] = empty
		}

		for _, cert := range drc.Spec.TLSConfig.Certificates {
			if cert.ClientCert.SecretKeyRef != nil {
				secretSet[cert.ClientCert.SecretKeyRef.LocalObjectReference.Name] = empty
			}
			if cert.ClientKey.SecretKeyRef != nil {
				secretSet[cert.ClientKey.SecretKeyRef.LocalObjectReference.Name] = empty
			}
		}
	}

	for _, df := range drc.Spec.DataFilters {
		if df.Transformer.ConfigMapKeyRef != nil {
			configMapSet[df.Transformer.ConfigMapKeyRef.Name] = empty
		}

		for _, dest := range df.AltDestinations {
			if dest.Transformer.ConfigMapKeyRef != nil {
				configMapSet[dest.Transformer.ConfigMapKeyRef.Name] = empty
			}

			if dest.Authorization.BodyData.SecretKeyRef != nil {
				secretSet[dest.Authorization.BodyData.SecretKeyRef.Name] = empty
			}

			secretSet[dest.Header.Secret.Name] = empty

			secretSet[dest.Authorization.Header.Secret.Name] = empty
		}
	}

	r.configMapsSet.Set(configMapSet)
	r.secretsSet.Set(secretSet)
}

type watchSet struct {
	items map[string]struct{}
	mu    sync.RWMutex
}

func (w *watchSet) Set(items map[string]struct{}) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.items = items
}

func (w *watchSet) Exists(item string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	_, ok := w.items[item]
	return ok
}
