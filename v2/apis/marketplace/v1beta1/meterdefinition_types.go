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
	"bytes"
	"errors"

	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/signer"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// MeterDefinitionSpec defines the desired metering spec
// +k8s:openapi-gen=true
type MeterDefinitionSpec struct {
	// Group defines the operator group of the meter
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:text"
	Group string `json:"group"`

	// Kind defines the primary CRD kind of the meter
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:text"
	Kind string `json:"kind"`

	// ResourceFilters provide filters that will be used to find the workload objects.
	// This is to find the exact resources the query is interested in. At least one must
	// be provided.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +kubebuilder:validation:MinItems:=1
	ResourceFilters []ResourceFilter `json:"resourceFilters"`

	// Meters are the definitions related to the metrics that you would like to monitor.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:text"
	// +patchMergeKey=metricId
	// +patchStrategy=merge
	Meters []MeterWorkload `json:"meters"`

	// InstalledBy is a reference to the CSV that install the meter
	// definition. This is used to determine an operator group.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:hidden"
	// +optional
	InstalledBy *common.NamespacedNameReference `json:"installedBy,omitempty"`
}

// MeterDefinitionReference is a more friendly form to set on the MeterReport providing
// the default meter definitions to use for the day.
type MeterDefinitionReference struct {
	// Name is the name of the MeterDefinition
	Name string `json:"name"`

	// Namespace is the namespace of the MeterDefinition
	Namespace string `json:"namespace"`

	// UID is the UID of the MeterDefinition
	UID types.UID `json:"uid,omitempty"`

	// ResourceVersion
	ResourceVersion string `json:"resourceVersion,omitempty"`

	// Spec is a copy of the meter definition spec, this can be left empty and later filled in by
	// the report tool.
	//
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	Spec *MeterDefinitionSpec `json:"spec,omitempty"`
}

func (meterdef *MeterDefinitionReference) ToPrometheusLabels() ([]*common.MeterDefPrometheusLabels, error) {
	if meterdef.Spec == nil {
		return []*common.MeterDefPrometheusLabels{}, errors.New("meter definition spec cannot be nil")
	}

	return meterdef.Spec.ToPrometheusLabels(string(meterdef.UID), meterdef.Name, meterdef.Namespace), nil
}

type WorkloadVertex string
type CSVNamespacedName common.NamespacedNameReference

const (
	WorkloadVertexOperatorGroup WorkloadVertex = "OperatorGroup"
	WorkloadVertexNamespace                    = "Namespace"

	ReconcileError                    status.ConditionType = "ReconcileError"
	MeterDefQueryPreviewSetupError    status.ConditionType = "QueryPreviewSetupError"
	MeterDefVerifyReportingSetupError status.ConditionType = "VerifyReportingSetupError"
)

type ResourceFilter struct {
	// Namespace is the filter to control which namespaces to look for your resources.
	// Default is always Operator Group (supported by OLM)
	Namespace *NamespaceFilter `json:"namespace,omitempty"`

	// OwnerCRD uses the owning CRD to filter resources.
	OwnerCRD *OwnerCRDFilter `json:"ownerCRD,omitempty"`

	// Label uses the resource annotations to find resources to monitor.
	Label *LabelFilter `json:"label,omitempty"`

	// Annotation uses the resource annotations to find resources to monitor.
	Annotation *AnnotationFilter `json:"annotation,omitempty"`

	// WorkloadType identifies the type of workload to look for. This can be
	// pod or service right now.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:select:Pod,urn:alm:descriptor:com.tectonic.ui:select:Service,urn:alm:descriptor:com.tectonic.ui:select:PersistentVolumeClaim"
	// +kubebuilder:validation:Enum:=Pod;Service;PersistentVolumeClaim
	WorkloadType common.WorkloadType `json:"workloadType"`
}

type NamespaceFilter struct {
	// UseOperatorGroup use your operator group for namespace filtering
	UseOperatorGroup bool `json:"useOperatorGroup"`

	// LabelSelector are used to filter to the correct workload.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

type OwnerCRDFilter struct {
	common.GroupVersionKind `json:",inline"`
}

type LabelFilter struct {
	// LabelSelector are used to filter to the correct workload.
	// +optional
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

type AnnotationFilter struct {
	// AnnotationSelector are used to filter to the correct workload.
	// +optional
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	AnnotationSelector *metav1.LabelSelector `json:"annotationSelector,omitempty"`
}

type MeterWorkload struct {
	// Metric is the id of the meter
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:text"
	Metric string `json:"metricId"`

	// Name of the metric for humans to read.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	Name string `json:"name,omitempty"`

	// Description is the overview of what the metric is providing for humans to read.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	Description string `json:"description,omitempty"`

	// WorkloadType identifies the type of workload to look for. This can be
	// pod or service right now.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:select:Pod,urn:alm:descriptor:com.tectonic.ui:select:Service,urn:alm:descriptor:com.tectonic.ui:select:PersistentVolumeClaim"
	// +kubebuilder:validation:Enum:=Pod;Service;PersistentVolumeClaim
	WorkloadType common.WorkloadType `json:"workloadType"`

	// MetricType identifies the type of metric this meter definition reports. Currently "billable", "license", "adoption", or "infrastructure".
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:select:adoption,urn:alm:descriptor:com.tectonic.ui:select:billable,urn:alm:descriptor:com.tectonic.ui:select:license,urn:alm:descriptor:com.tectonic.ui:select:infrastructure"
	// +optional
	// +kubebuilder:validation:Enum:=billable;license;adoption;infrastructure
	MetricType common.MetricType `json:"metricType,omitempty"`

	// Group is the set of label fields returned by query to aggregate on.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	// +listType:=set
	GroupBy []string `json:"groupBy,omitempty"`

	// Labels to filter out automatically.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	// +listType:=set
	Without []string `json:"without,omitempty"`

	// Aggregation to use with the query
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:select:sum,urn:alm:descriptor:com.tectonic.ui:select:min,urn:alm:descriptor:com.tectonic.ui:select:max,urn:alm:descriptor:com.tectonic.ui:select:avg"
	// +kubebuilder:validation:Enum:=sum;min;max;avg;group
	Aggregation string `json:"aggregation"`

	// Period is the amount of time to segment the data into. Default is 1h.
	// +optional
	Period *metav1.Duration `json:"period,omitempty"`

	// Query to use for prometheus to find the metrics
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	Query string `json:"query"`

	// Label is the usage metrics key field on the report.
	// Defaults to the metricId field if not provided
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	Label string `json:"label,omitempty"`

	// Unit is the unit of the metrics.
	// Defaults to the metricId field if not provided
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	Unit string `json:"unit,omitempty"`

	// DateLabelOverride provides a means of overriding the date returned for the metric using a label.
	// This is to handle cases where the metric is a constant that is calculated.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	DateLabelOverride string `json:"dateLabelOverride,omitempty"`

	// ValueLabelOverride provides a means of overriding the value returned for the metric using a label.
	// This is to handle cases where the metric is a constant that is calculated.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +optional
	ValueLabelOverride string `json:"valueLabelOverride,omitempty"`
}

// MeterDefinitionStatus defines the observed state of MeterDefinition
// +k8s:openapi-gen=true
// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
type MeterDefinitionStatus struct {
	// Conditions represent the latest available observations of an object's state
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors.x-descriptors="urn:alm:descriptor:io.kubernetes.conditions"
	// +optional
	Conditions status.Conditions `json:"conditions,omitempty"`

	// WorkloadResources is the list of resources discovered by
	// this meter definition
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	WorkloadResources []common.WorkloadResource `json:"workloadResource,omitempty"`

	// Results is a list of Results that get returned from a query to prometheus
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	Results []common.Result `json:"results,omitempty"`
}

// MeterDefinition defines the meter workloads used to enable pay for
// use billing.
// +kubebuilder:object:root=true
//
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:resource:path=meterdefinitions,scope=Namespaced
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="Meter Definitions"
// +genclient
type MeterDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MeterDefinitionSpec   `json:"spec,omitempty"`
	Status MeterDefinitionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MeterDefinitionList contains a list of MeterDefinition
type MeterDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MeterDefinition `json:"items"`
}

// Hub function to create conversion
func (*MeterDefinition) Hub() {}

var _ conversion.Hub = &MeterDefinition{}

func init() {
	SchemeBuilder.Register(&MeterDefinition{}, &MeterDefinitionList{})
}

func (meterdef *MeterDefinition) ToPrometheusLabels() []*common.MeterDefPrometheusLabels {
	return meterdef.Spec.ToPrometheusLabels(string(meterdef.UID), meterdef.Name, meterdef.Namespace)
}

func (meterdefspec *MeterDefinitionSpec) ToPrometheusLabels(uid, name, namespace string) []*common.MeterDefPrometheusLabels {
	allMdefs := []*common.MeterDefPrometheusLabels{}

	for _, meter := range meterdefspec.Meters {
		var period *common.MetricPeriod

		if meter.Period != nil {
			period = &common.MetricPeriod{Duration: meter.Period.Duration}
		}

		obj := &common.MeterDefPrometheusLabels{
			UID:                string(uid),
			MeterDefName:       string(name),
			MeterDefNamespace:  string(namespace),
			MeterKind:          meterdefspec.Kind,
			WorkloadName:       meter.Metric,
			Metric:             meter.Metric,
			MetricGroupBy:      common.JSONArray(meter.GroupBy),
			MeterGroup:         meterdefspec.Group,
			MetricQuery:        meter.Query,
			MetricPeriod:       period,
			Label:              meter.Label,
			Unit:               meter.Unit,
			DisplayName:        meter.Name,
			MetricWithout:      common.JSONArray(meter.Without),
			WorkloadType:       meter.WorkloadType,
			MetricAggregation:  meter.Aggregation,
			MeterDescription:   meter.Description,
			DateLabelOverride:  meter.DateLabelOverride,
			ValueLabelOverride: meter.ValueLabelOverride,
			MetricType:         meter.MetricType,
		}

		allMdefs = append(allMdefs, obj)
	}

	return allMdefs
}

func (meterdef *MeterDefinition) BuildMeterDefinitionFromString(
	meterdefString, name, namespace, nameLabel, namespaceLabel string) error {
	data := []byte(meterdefString)

	err := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 100).Decode(meterdef)
	if err != nil {
		return err
	}

	csvInfo := make(map[string]string)
	csvInfo[nameLabel] = name
	csvInfo[namespaceLabel] = namespace
	meterdef.SetAnnotations(csvInfo)

	meterdef.Namespace = namespace
	meterdef.Spec.InstalledBy = &common.NamespacedNameReference{
		Name:      name,
		Namespace: namespace,
	}

	return nil
}

func (meterdef *MeterDefinition) IsSigned() bool {
	annotations := meterdef.GetAnnotations()
	publicKey := annotations["marketplace.redhat.com/publickey"]
	signature := annotations["marketplace.redhat.com/signature"]
	if (len(publicKey) != 0) && (len(signature) != 0) {
		return true
	}
	return false
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

var (
	VerifyReportingErrorCondition = status.Condition{
		Type:   MeterDefVerifyReportingSetupError,
		Reason: "VerifyReportingError",
		Status: corev1.ConditionTrue,
	}

	PreviewErrorCondition = status.Condition{
		Type:   MeterDefQueryPreviewSetupError,
		Reason: "PreviewError",
		Status: corev1.ConditionTrue,
	}
)
