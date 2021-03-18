/*
Copyright 2020 IBM Co..

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

package v1beta1

import (
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/signer"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var meterdefinitionlog = logf.Log.WithName("meterdefinition-resource")

func (r *MeterDefinition) SetupWebhookWithManager(mgr ctrl.Manager) error {
	bldr := ctrl.NewWebhookManagedBy(mgr).For(r)
	return bldr.Complete()
}

// +kubebuilder:webhook:path=/mutate-marketplace-redhat-com-v1beta1-meterdefinition,mutating=true,failurePolicy=fail,sideEffects=None,groups=marketplace.redhat.com,resources=meterdefinitions,verbs=create;update,versions=v1beta1,name=mmeterdefinition.marketplace.redhat.com

var _ webhook.Defaulter = &MeterDefinition{}

// // Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *MeterDefinition) Default() {
	meterdefinitionlog.Info("default", "name", r.Name, "mdef", r)
}

// Disabled for now
// +kubebuilder:webhook:path=/validate-marketplace-redhat-com-v1beta1-meterdefinition,mutating=false,failurePolicy=fail,sideEffects=None,groups=marketplace.redhat.com,resources=meterdefinitions,verbs=create;update,versions=v1beta1,name=vmeterdefinition.marketplace.redhat.com

var _ webhook.Validator = &MeterDefinition{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *MeterDefinition) ValidateCreate() error {
	var allErrs field.ErrorList
	meterdefinitionlog.Info("validate create", "name", r.Name)

	for _, resource := range r.Spec.ResourceFilters {
		if resource.OwnerCRD == nil &&
			resource.Annotation == nil &&
			resource.Label == nil {

			allErrs = append(allErrs, field.Required(
				field.NewPath("spec").Child("meters").Child("resourceFilters"),
				"one of resource filter owner crd, annotation, or label must be provided",
			))
		}
	}

	if r.IsSigned() {
		// Check required fields which may not be mutated on signed MeterDefinitions
		for _, resourceFilter := range r.Spec.ResourceFilters {
			if resourceFilter.Namespace == nil {
				allErrs = append(allErrs, field.Required(
					field.NewPath("spec").Child("resourceFilters"),
					"namespace must be provided",
				))
			}
		}
		for _, meter := range r.Spec.Meters {
			if len(meter.Aggregation) == 0 {
				allErrs = append(allErrs, field.Required(
					field.NewPath("spec").Child("meters"),
					"aggregation must be provided",
				))
			}
			if meter.Period == nil {
				allErrs = append(allErrs, field.Required(
					field.NewPath("spec").Child("meters"),
					"period must be provided",
				))
			}
		}

		err := r.ValidateSignature()
		if err != nil {
			allErrs = append(allErrs, field.InternalError(
				field.NewPath("metadata").Child("annotations"),
				err,
			))
		}
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "marketplace.redhat.com", Kind: "MeterDefinition"},
		r.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *MeterDefinition) ValidateUpdate(old runtime.Object) error {
	var allErrs field.ErrorList
	meterdefinitionlog.Info("validate update", "name", r.Name)

	for _, resource := range r.Spec.ResourceFilters {
		if resource.OwnerCRD == nil &&
			resource.Annotation == nil &&
			resource.Label == nil {

			allErrs = append(allErrs, field.Required(
				field.NewPath("spec").Child("meters").Child("resourceFilters"),
				"one of resource filter owner crd, annotation, or label must be provided",
			))
		}
	}

	if r.IsSigned() {
		// Check required fields which may not be mutated on signed MeterDefinitions
		for _, resourceFilter := range r.Spec.ResourceFilters {
			if resourceFilter.Namespace == nil {
				allErrs = append(allErrs, field.Required(
					field.NewPath("spec").Child("resourceFilters"),
					"namespace must be provided",
				))
			}
		}
		for _, meter := range r.Spec.Meters {
			if len(meter.Aggregation) == 0 {
				allErrs = append(allErrs, field.Required(
					field.NewPath("spec").Child("meters"),
					"aggregation must be provided",
				))
			}
			if meter.Period == nil {
				allErrs = append(allErrs, field.Required(
					field.NewPath("spec").Child("meters"),
					"period must be provided",
				))
			}
		}

		err := r.ValidateSignature()
		if err != nil {
			allErrs = append(allErrs, field.InternalError(
				field.NewPath("metadata").Child("annotations"),
				err,
			))
		}
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "marketplace.redhat.com", Kind: "MeterDefinition"},
		r.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *MeterDefinition) ValidateDelete() error {
	meterdefinitionlog.Info("validate delete", "name", r.Name)

	return nil
}

func (r *MeterDefinition) ValidateSignature() error {
	uMeterDef := unstructured.Unstructured{}

	uContent, err := runtime.DefaultUnstructuredConverter.ToUnstructured(r)
	if err != nil {
		return err
	}

	uMeterDef.SetUnstructuredContent(uContent)

	caCert, err := signer.CertificateFromAssets()
	if err != nil {
		return err
	}

	return signer.VerifySignature(uMeterDef, caCert)
}
