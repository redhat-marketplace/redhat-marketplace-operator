// Copyright 2021 IBM Corp.
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

package reporter

import (
	"fmt"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter/schema/common"
	schemav1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter/schema/v1alpha1"
	schemav2alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter/schema/v2alpha1"
	writerv1 "github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter/writer/v1"
	writerv2 "github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter/writer/v2"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
)

func ProvideWriter(
	config *Config,
	MktConfig *marketplacev1alpha1.MarketplaceConfig,
	logger logr.Logger,
) (common.ReportWriter, error) {
	switch config.ReporterSchema {
	case "v1alpha1":
		return &writerv1.ReportWriter{MktConfig: MktConfig, Logger: logger}, nil
	case "v2alpha1":
		return &writerv2.ReportWriter{MktConfig: MktConfig, Logger: logger}, nil
	default:
		return nil, errors.New(fmt.Sprintf("Unsupported reporterSchema: %s", config.ReporterSchema))
	}
}

func ProvideDataBuilder(
	config *Config,
	logger logr.Logger,
) (common.DataBuilder, error) {
	switch config.ReporterSchema {
	case "v1alpha1":
		return common.DataBuilderFunc(func() common.SchemaMetricBuilder {
			return &schemav1alpha1.MarketplaceReportDataBuilder{}
		}), nil
	case "v2alpha1":
		return common.DataBuilderFunc(func() common.SchemaMetricBuilder {
			return &schemav2alpha1.MarketplaceReportDataBuilder{}
		}), nil
	default:
		return nil, errors.New(fmt.Sprintf("Unsupported reporterSchema: %s", config.ReporterSchema))
	}
}
