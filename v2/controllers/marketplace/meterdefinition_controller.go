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
	"strings"
	"time"

	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	v1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/inject"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/patch"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/reconcileutils"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const meterDefinitionFinalizer = "meterdefinition.finalizer.marketplace.redhat.com"

const (
	MeteredResourceAnnotationKey = "marketplace.redhat.com/meteredUIDs"
)

// blank assignment to verify that ReconcileMeterDefinition implements reconcile.Reconciler
var _ reconcile.Reconciler = &MeterDefinitionReconciler{}

// MeterDefinitionReconciler reconciles a MeterDefinition object
type MeterDefinitionReconciler struct {
	// This Client, initialized using mgr.Client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	Client client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	cc      ClientCommandRunner
	patcher patch.Patcher
}

func (r *MeterDefinitionReconciler) Inject(injector *inject.Injector) inject.SetupWithManager {
	injector.SetCustomFields(r)
	return r
}

func (r *MeterDefinitionReconciler) InjectCommandRunner(ccp ClientCommandRunner) error {
	r.cc = ccp
	return nil
}

func (r *MeterDefinitionReconciler) InjectPatch(p patch.Patcher) error {
	r.patcher = p
	return nil
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func (r *MeterDefinitionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create a new controller
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.MeterDefinition{}).
		Watches(&source.Kind{Type: &v1alpha1.MeterDefinition{}}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}

// Reconcile reads that state of the cluster for a MeterDefinition object and makes changes based on the state read
// and what is in the MeterDefinition.Spec
func (r *MeterDefinitionReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling MeterDefinition")

	cc := r.cc

	// Fetch the MeterDefinition instance
	instance := &v1beta1.MeterDefinition{}
	result, _ := cc.Do(context.TODO(),
		GetAction(request.NamespacedName, instance),
	)

	if !result.Is(Continue) {
		if result.Is(NotFound) {
			reqLogger.Info("MeterDef resource not found. Ignoring since object must be deleted.")
			return reconcile.Result{}, nil
		}

		if result.Is(Error) {
			reqLogger.Error(result.GetError(), "Failed to get MeterDef.")
		}

		return result.Return()
	}

	reqLogger.Info("Found instance", "instance", instance.Name)

	result, _ = cc.Do(
		context.TODO(),
		Call(func() (ClientAction, error) {
			var queue bool

			// Set the Status Condition of signature signing
			if instance.IsSigned() {
				// Check the signature again, even though the admission webhook should have
				err := instance.ValidateSignature()
				if err != nil {
					// This could occur after the admission webhook if a signed v1alpha1 is converted to v1beta1.
					// However, signing not introduced until v1beta1, so this is unlikely.
					queue = queue || instance.Status.Conditions.SetCondition(common.MeterDefConditionSignatureVerificationFailed)
				} else {
					queue = queue || instance.Status.Conditions.SetCondition(common.MeterDefConditionSignatureVerified)
				}

			} else {
				// Unsigned, Unverified
				queue = queue || instance.Status.Conditions.SetCondition(common.MeterDefConditionSignatureUnverified)
			}

			switch {
			case instance.Status.Conditions.IsUnknownFor(common.MeterDefConditionTypeHasResult):
				fallthrough
			case len(instance.Status.WorkloadResources) == 0:
				queue = queue || instance.Status.Conditions.SetCondition(common.MeterDefConditionNoResults)
			case len(instance.Status.WorkloadResources) > 0:
				queue = queue || instance.Status.Conditions.SetCondition(common.MeterDefConditionHasResults)
			}

			if !queue {
				return nil, nil
			}

			return UpdateAction(instance, UpdateStatusOnly(true)), nil
		}),
	)
	if result.Is(Error) {
		reqLogger.Error(result.GetError(), "Failed to update status.")
	}

	reqLogger.Info("finished reconciling")
	return reconcile.Result{RequeueAfter: time.Minute * 1}, nil
}

func (r *MeterDefinitionReconciler) finalizeMeterDefinition(req *v1alpha1.MeterDefinition) (reconcile.Result, error) {
	var err error

	// TODO: add finalizers

	req.SetFinalizers(utils.RemoveKey(req.GetFinalizers(), meterDefinitionFinalizer))
	err = r.Client.Update(context.TODO(), req)
	if err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

// addFinalizer adds finalizers to the MeterDefinition CR
func (r *MeterDefinitionReconciler) addFinalizer(instance *v1alpha1.MeterDefinition) error {
	r.Log.Info("Adding Finalizer to %s/%s", instance.Name, instance.Namespace)
	instance.SetFinalizers(append(instance.GetFinalizers(), meterDefinitionFinalizer))

	err := r.Client.Update(context.TODO(), instance)
	if err != nil {
		r.Log.Error(err, "Failed to update RazeeDeployment with the Finalizer %s/%s", instance.Name, instance.Namespace)
		return err
	}
	return nil
}

func labelsForServiceMonitor(name, namespace string) map[string]string {
	return map[string]string{
		"marketplace.redhat.com/metered":                  "true",
		"marketplace.redhat.com/deployed":                 "true",
		"marketplace.redhat.com/metered.kind":             "ServiceMonitor",
		"marketplace.redhat.com/serviceMonitor.Name":      name,
		"marketplace.redhat.com/serviceMonitor.Namespace": namespace,
	}
}

func labelsForKubeStateMonitor(name, namespace string) map[string]string {
	return map[string]string{
		"marketplace.redhat.com/metered":                   "true",
		"marketplace.redhat.com/deployed":                  "true",
		"marketplace.redhat.com/metered.kind":              "ServiceMonitor",
		"marketplace.redhat.com/meterDefinition.namespace": namespace,
		"marketplace.redhat.com/meterDefinition.name":      name,
	}
}

func makeRelabelConfig(source []string, action, target string) *monitoringv1.RelabelConfig {
	return &monitoringv1.RelabelConfig{
		SourceLabels: source,
		TargetLabel:  target,
		Action:       action,
	}
}

func makeRelabelReplaceConfig(source []string, target, regex, replacement string) *monitoringv1.RelabelConfig {
	return &monitoringv1.RelabelConfig{
		SourceLabels: source,
		TargetLabel:  target,
		Action:       "replace",
		Regex:        regex,
		Replacement:  replacement,
	}
}

func makeRelabelKeepConfig(source []string, regex string) *monitoringv1.RelabelConfig {
	return &monitoringv1.RelabelConfig{
		SourceLabels: source,
		Action:       "keep",
		Regex:        regex,
	}

}
func labelsToRegex(labels []string) string {
	return fmt.Sprintf("(%s)", strings.Join(labels, "|"))
}
