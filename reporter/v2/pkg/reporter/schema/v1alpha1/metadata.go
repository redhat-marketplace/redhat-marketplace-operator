package v1alpha1

import (
	"github.com/google/uuid"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter/schema/common"
)

type ReportMetadata struct {
	ReportID       uuid.UUID                            `json:"report_id"`
	Source         uuid.UUID                            `json:"source"`
	SourceMetadata SourceMetadata                       `json:"source_metadata"`
	ReportSlices   map[common.ReportSliceKey]ReportSlicesValue `json:"report_slices"`
}

type SourceMetadata struct {
	RhmClusterID   string                   `json:"rhmClusterId" mapstructure:"rhmClusterId"`
	RhmAccountID   string                   `json:"rhmAccountId" mapstructure:"rhmAccountId"`
	RhmEnvironment common.ReportEnvironment `json:"rhmEnvironment,omitempty" mapstructure:"rhmEnvironment,omitempty"`
	Version        string                   `json:"version,omitempty" mapstructure:"version,omitempty"`
}

type ReportSlicesValue struct {
	NumberMetrics int `json:"number_metrics"`
}
