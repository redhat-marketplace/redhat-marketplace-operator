package v1alpha1

import (
	"time"

	"emperror.dev/errors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this MeterDefinition to the Hub version (v1beta1).
func (src *MeterDefinition) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.MeterDefinition)

	meters := []v1beta1.MeterWorkload{}

	hrDuration, _ := time.ParseDuration("1h")
	metaHRDuration := metav1.Duration{Duration: hrDuration}

	var namespaceFilters *v1beta1.NamespaceFilter

	switch src.Spec.WorkloadVertexType {
	case WorkloadVertexOperatorGroup:
		namespaceFilters = &v1beta1.NamespaceFilter{
			UseOperatorGroup: true,
		}
	case WorkloadVertexNamespace:
		namespaceFilters = &v1beta1.NamespaceFilter{
			UseOperatorGroup: false,
			LabelSelector:    src.Spec.VertexLabelSelector,
		}
	}

	resourceFilters := []v1beta1.ResourceFilter{}

	for _, workload := range src.Spec.Workloads {
		filters := v1beta1.ResourceFilter{}

		if workload.AnnotationSelector != nil {
			filters.Annotation = &v1beta1.AnnotationFilter{
				AnnotationSelector: workload.AnnotationSelector,
			}
		}

		if workload.LabelSelector != nil {
			filters.Label = &v1beta1.LabelFilter{
				LabelSelector: workload.LabelSelector,
			}
		}

		filters.Namespace = namespaceFilters

		workloadType, err := ConvertWorkloadTypeBeta(workload.WorkloadType)

		if err != nil {
			return err
		}

		filters.WorkloadType = workloadType

		if workload.OwnerCRD != nil {
			filters.OwnerCRD = &v1beta1.OwnerCRDFilter{
				GroupVersionKind: *workload.OwnerCRD,
			}
		}

		resourceFilters = append(resourceFilters, filters)

		for _, metricLabel := range workload.MetricLabels {
			meters = append(meters, v1beta1.MeterWorkload{
				Aggregation:  metricLabel.Aggregation,
				GroupBy:      []string{},
				WorkloadType: workloadType,
				Metric:       metricLabel.Label,
				Name:         workload.Name,
				Period:       &metaHRDuration,
				Query:        metricLabel.Query,
			})
		}
	}

	dst.ObjectMeta = src.ObjectMeta
	dst.Spec.Group = string(src.Spec.Group)
	dst.Spec.Kind = string(src.Spec.Kind)
	dst.Spec.InstalledBy = src.Spec.InstalledBy
	dst.Spec.Meters = meters
	dst.Spec.ResourceFilters = resourceFilters
	dst.Status.Conditions = src.Status.Conditions
	dst.Status.WorkloadResources = src.Status.WorkloadResources
	return nil
}

var _ conversion.Convertible = &MeterDefinition{}

// ConvertFrom converts from the Hub version (v1beta1) to this version.
func (dst *MeterDefinition) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.MeterDefinition)
	workloads := []Workload{}

	if len(src.Spec.ResourceFilters) == 0 {
		return errors.NewWithDetails("failed to convert to v1alpha1", "no resource filters available")
	}

	filter := src.Spec.ResourceFilters[0]

	if filter.Namespace != nil && filter.Namespace.UseOperatorGroup == true {
		dst.Spec.WorkloadVertexType = WorkloadVertexOperatorGroup
	}

	if filter.Namespace != nil && filter.Namespace.UseOperatorGroup == false {
		dst.Spec.WorkloadVertexType = WorkloadVertexNamespace
		dst.Spec.VertexLabelSelector = filter.Namespace.LabelSelector
	}

	resourceMap := map[WorkloadType]v1beta1.ResourceFilter{}

	for _, filter := range src.Spec.ResourceFilters {
		workloadType, _ := ConvertWorkloadTypeAlpha(filter.WorkloadType)

		if _, ok := resourceMap[workloadType]; !ok {
			resourceMap[workloadType] = filter
		}
	}

	for _, meter := range src.Spec.Meters {
		workloadType, _ := ConvertWorkloadTypeAlpha(meter.WorkloadType)

		workload := Workload{
			Name: meter.Name,
			MetricLabels: []MeterLabelQuery{
				{
					Aggregation: meter.Aggregation,
					Label:       meter.Metric,
					Query:       meter.Query,
				},
			},
			WorkloadType: workloadType,
		}

		filter := resourceMap[workloadType]

		if filter.Annotation != nil {
			workload.AnnotationSelector = filter.Annotation.AnnotationSelector
		}

		if filter.Label != nil {
			workload.LabelSelector = filter.Label.LabelSelector
		}

		if filter.OwnerCRD != nil {
			workload.OwnerCRD = &filter.OwnerCRD.GroupVersionKind
		}

		workloads = append(workloads, workload)
	}

	dst.ObjectMeta = src.ObjectMeta
	dst.Spec.Group = src.Spec.Group
	dst.Spec.Kind = src.Spec.Kind
	dst.Spec.Workloads = workloads
	dst.Spec.InstalledBy = src.Spec.InstalledBy
	dst.Status.Conditions = src.Status.Conditions
	dst.Status.WorkloadResources = src.Status.WorkloadResources

	return nil
}

func ConvertWorkloadTypeAlpha(typeIn interface{}) (WorkloadType, error) {
	workloadType := v1beta1.WorkloadTypePod

	switch v := typeIn.(type) {
	case WorkloadType:
		return v, nil
	case v1beta1.WorkloadType:
		workloadType = v
	case string:
		workloadType = v1beta1.WorkloadType(v)
	}

	switch workloadType {
	case v1beta1.WorkloadTypePod:
		return WorkloadTypePod, nil
	case v1beta1.WorkloadTypePVC:
		return WorkloadTypePVC, nil
	case v1beta1.WorkloadTypeService:
		return WorkloadTypeService, nil
	default:
		return WorkloadTypePod, errors.NewWithDetails("workload type cannot be generated from workload type", "type", typeIn)
	}
}

func ConvertWorkloadTypeBeta(typeIn interface{}) (v1beta1.WorkloadType, error) {
	workloadType := WorkloadTypePod

	switch v := typeIn.(type) {
	case v1beta1.WorkloadType:
		return v, nil
	case WorkloadType:
		workloadType = v
	case string:
		workloadType = WorkloadType(v)
	}

	switch workloadType {
	case WorkloadTypePod:
		return v1beta1.WorkloadTypePod, nil
	case WorkloadTypePVC:
		return v1beta1.WorkloadTypePVC, nil
	case WorkloadTypeService:
		return v1beta1.WorkloadTypeService, nil
	case WorkloadTypeServiceMonitor:
		return v1beta1.WorkloadTypeService, nil
	default:
		return v1beta1.WorkloadTypePod, errors.NewWithDetails("workload type cannot be generated from workload type", "type", typeIn)
	}
}
