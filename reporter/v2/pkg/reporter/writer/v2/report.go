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

package v2

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/meirf/gopart"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter/schema/common"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter/schema/v2alpha1"
	schemav2alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter/schema/v2alpha1"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/version"
)

type ReportWriter struct {
	MktConfig *marketplacev1alpha1.MarketplaceConfig
	Logger    logr.Logger
}

func (r *ReportWriter) WriteReport(
	source uuid.UUID,
	metrics map[string]common.SchemaMetricBuilder,
	outputDirectory string,
	partitionSize int,
) ([]string, error) {
	logger := r.Logger
	env := common.ReportProductionEnv
	envAnnotation, ok := r.MktConfig.Annotations["marketplace.redhat.com/environment"]

	if ok && envAnnotation == common.ReportSandboxEnv.String() {
		env = common.ReportSandboxEnv
	}

	metadata := schemav2alpha1.SourceMetadata{
		RhmAccountID:   r.MktConfig.Spec.RhmAccountID,
		RhmClusterID:   r.MktConfig.Spec.ClusterUUID,
		RhmEnvironment: env,
		Version:        version.Version,
		ReportVersion:  schemav2alpha1.Version,
	}

	metricsArr := make([]common.SchemaMetricBuilder, 0, len(metrics))

	filedir := filepath.Join(outputDirectory, source.String())
	err := os.Mkdir(filedir, 0755)

	if err != nil && !errors.Is(err, os.ErrExist) {
		return []string{}, errors.Wrap(err, "error creating directory")
	}

	for _, v := range metrics {
		metricsArr = append(metricsArr, v)
	}

	filenames := []string{}
	reportErrors := []error{}

	for idxRange := range gopart.Partition(len(metricsArr), partitionSize) {
		metricReport := &schemav2alpha1.MarketplaceReportSlice{}
		reportSliceID := common.ReportSliceKey(uuid.New())
		metricReport.Metadata = &metadata

		for _, builder := range metricsArr[idxRange.Low:idxRange.High] {
			metric, err := builder.Build()

			if err != nil {
				reportErrors = append(reportErrors, err)
			} else {
				metricReport.Metrics = append(metricReport.Metrics, metric.(*v2alpha1.MarketplaceReportData))
			}

		}

		marshallBytes, err := json.Marshal(metricReport)
		logger.V(4).Info(string(marshallBytes))
		if err != nil {
			logger.Error(err, "failed to marshal metrics report", "report", metricReport)
			return nil, err
		}

		filename := filepath.Join(
			filedir,
			fmt.Sprintf("%s.json", reportSliceID.String()))

		err = ioutil.WriteFile(
			filename,
			marshallBytes,
			0600)

		if err != nil {
			logger.Error(err, "failed to write file", "file", filename)
			return nil, errors.Wrap(err, "failed to write file")
		}

		filenames = append(filenames, filename)
	}

	manifest := &schemav2alpha1.Manifest{
		Type:    schemav2alpha1.AccountMetrics,
		Version: "1",
	}

	marshallBytes, err := json.Marshal(manifest)
	if err != nil {
		logger.Error(err, "failed to marshal report manifest", "manifest", manifest)
		return nil, err
	}

	filename := filepath.Join(filedir, "manifest.json")

	err = ioutil.WriteFile(filename, marshallBytes, 0600)
	if err != nil {
		logger.Error(err, "failed to write file", "file", filename)
		return nil, err
	}

	filenames = append(filenames, filename)

	return filenames, errors.Combine(reportErrors...)
}
