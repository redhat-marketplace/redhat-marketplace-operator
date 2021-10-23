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

package upload

import (
	"context"
	"os"
	"time"

	"github.com/gotidy/ptr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter"
	"github.com/spf13/cobra"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("reporter_upload_cmd")

var deployedNamespace, dataServiceTokenFile, dataServiceCertFile string
var uploadTargets []string
var retry int

var UploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload the reports from dataservice to redhat-insights",
	Long:  `Upload the reports from dataservice to redhat-insights`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Info("running the upload command")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		tmpDir := os.TempDir()

		uploadTarget := reporter.MustParseUploaderTarget("redhat-insights")
		log.Info("upload target", "target set to", uploadTarget.Name())

		cfg := &reporter.Config{
			OutputDirectory:      tmpDir,
			Retry:                ptr.Int(retry),
			DataServiceTokenFile: dataServiceTokenFile,
			DataServiceCertFile:  dataServiceCertFile,
			UploaderTargets:      reporter.UploaderTargets{uploadTarget},
			DeployedNamespace:    deployedNamespace,
		}
		cfg.SetDefaults()

		uploadTask, err := reporter.NewUploadTask(
			ctx,
			cfg,
		)

		if err != nil {
			log.Error(err, "couldn't initialize upload task")
			os.Exit(1)
		}

		err = uploadTask.Run()
		if err != nil {
			log.Error(err, "error running upload task")
			os.Exit(1)
		}

		os.Exit(0)
	},
}

func init() {
	UploadCmd.Flags().StringVar(&dataServiceTokenFile, "dataServiceTokenFile", "", "token file for the data service")
	UploadCmd.Flags().StringVar(&dataServiceCertFile, "dataServiceCertFile", "", "cert file for the data service")
	UploadCmd.Flags().StringSliceVar(&uploadTargets, "uploadTargets", []string{"redhat-insights"}, "comma seperated list of targets to upload to")
	UploadCmd.Flags().IntVar(&retry, "retry", 3, "number of retries")
	UploadCmd.Flags().StringVar(&deployedNamespace, "deployedNamespace", "openshift-redhat-marketplace", "namespace where the rhm operator is deployed")
}
