package v1alpha1

import (
	"time"

	"emperror.dev/errors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this CronJob to the Hub version (v1).
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

		workloadType, err := ConvertWorkloadType(workload.WorkloadType)

		if err != nil {
			return err
		}

		filters.WorkloadType = workloadType

		if workload.OwnerCRD != nil {
			filters.OwnerCRD = &v1beta1.OwnerCRDFilter{
				GroupVersionKind: *workload.OwnerCRD,
			}
		}

		for _, metricLabel := range workload.MetricLabels {
			meters = append(meters, v1beta1.MeterWorkload{
				Aggregation:     metricLabel.Aggregation,
				GroupBy:         []string{},
				ResourceFilters: filters,
				Metric:          metricLabel.Label,
				Name:            workload.Name,
				Period:          metaHRDuration,
				Query:           metricLabel.Query,
			})
		}
	}

	dst.Spec.Group = string(src.Spec.Group)
	dst.Spec.Kind = string(src.Spec.Kind)
	dst.Spec.InstalledBy = src.Spec.InstalledBy
	dst.Spec.Meters = meters
	return nil
}

var _ conversion.Convertible = &MeterDefinition{}

// ConvertFrom converts from the Hub version (v1) to this version.
func (dst *MeterDefinition) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.MeterDefinition)

	dst.Spec.Group = src.Spec.Group
	dst.Spec.Kind = src.Spec.Kind
	dst.Spec.InstalledBy = src.Spec.InstalledBy

	workloads := []Workload{}

	for _, meter := range src.Spec.Meters {
		workload := Workload{
			Name: meter.Name,
			MetricLabels: []MeterLabelQuery{
				{
					Aggregation: meter.Aggregation,
					Label:       meter.Metric,
					Query:       meter.Query,
				},
			},
		}

		if meter.ResourceFilters.Annotation != nil {
			workload.AnnotationSelector = meter.ResourceFilters.Annotation.AnnotationSelector
		}

		if meter.ResourceFilters.Label != nil {
			workload.LabelSelector = meter.ResourceFilters.Label.LabelSelector
		}

		if meter.ResourceFilters.Namespace != nil && meter.ResourceFilters.Namespace.UseOperatorGroup == true {
			dst.Spec.WorkloadVertexType = WorkloadVertexOperatorGroup
		}

		if meter.ResourceFilters.Namespace != nil && meter.ResourceFilters.Namespace.UseOperatorGroup == false {
			dst.Spec.WorkloadVertexType = WorkloadVertexNamespace
			dst.Spec.VertexLabelSelector = meter.ResourceFilters.Namespace.LabelSelector
		}

		workloads = append(workloads, workload)
	}

	return nil
}

func ConvertWorkloadType(typeIn interface{}) (v1beta1.WorkloadType, error) {
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
