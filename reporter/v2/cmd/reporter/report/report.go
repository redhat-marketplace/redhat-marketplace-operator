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

package report

import (
	"context"
	"os"
	"time"

	"emperror.dev/errors"
	"github.com/gotidy/ptr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter"
	"github.com/spf13/cobra"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("reporter_report_cmd")

var name, namespace, cafile, tokenFile,localFilePath,deployedNamespace,dataServiceTokenFile,dataServiceCertFile string
var uploadTargets []string
var local, upload bool
var retry int

var ReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Run the report",
	Long:  `Runs the report. Takes it name and namespace as args`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Info("running the report command")

		if name == "" || namespace == "" {
			log.Error(errors.New("name or namespace not provided"), "namespace or name not provided")
			os.Exit(1)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		tmpDir := os.TempDir()

		for _,uploadTarget := range uploadTargets {
			uploadTarget := reporter.MustParseUploaderTarget(uploadTarget)
			log.Info("upload target","target set to",uploadTarget.Name())
	
			switch v := uploadTarget.(type) {
			case *reporter.LocalFilePathUploader:
				v.LocalFilePath = localFilePath
			}
	
			cfg := &reporter.Config{
				OutputDirectory: tmpDir,
				Retry:           ptr.Int(retry),
				CaFile:          cafile,
				TokenFile:       tokenFile,
				DataServiceTokenFile: dataServiceTokenFile,
				DataServiceCertFile: dataServiceCertFile,
				Local:           local,
				Upload:          upload,
				UploaderTarget:  uploadTarget,
				DeployedNamespace: deployedNamespace,
			}
			cfg.SetDefaults()
	
			task, err := reporter.NewTask(
				ctx,
				reporter.ReportName{Namespace: namespace, Name: name},
				cfg,
			)
	
			if err != nil {
				log.Error(err, "couldn't initialize task")
				os.Exit(1)
			}
	
			err = task.Run()
			if err != nil {
				log.Error(err, "error running task")
				os.Exit(1)
			}
		}

		os.Exit(0)
	},
}

func init() {
	ReportCmd.Flags().StringVar(&name, "name", "", "name of the report")
	ReportCmd.Flags().StringVar(&namespace, "namespace", "", "namespace of the report")
	ReportCmd.Flags().StringVar(&cafile, "cafile", "", "cafile for prometheus")
	ReportCmd.Flags().StringVar(&tokenFile, "tokenfile", "", "token file for prometheus")
	ReportCmd.Flags().StringVar(&dataServiceTokenFile, "dataServiceTokenFile", "", "token file for the data service")
	ReportCmd.Flags().StringVar(&dataServiceCertFile, "dataServiceCertFile", "", "cert file for the data service")
	ReportCmd.Flags().StringSliceVar(&uploadTargets, "uploadTargets", []string{"redhat-insights"}, "comma seperated list of targets to upload to")
	ReportCmd.Flags().StringVar(&localFilePath, "localFilePath", ".", "target to upload to")
	ReportCmd.Flags().BoolVar(&local, "local", false, "run locally")
	ReportCmd.Flags().BoolVar(&upload, "upload", true, "to upload the payload")
	ReportCmd.Flags().IntVar(&retry, "retry", 3, "number of retries")
	ReportCmd.Flags().StringVar(&deployedNamespace, "deployedNamespace", "openshift-redhat-marketplace", "namespace where the rhm operator is deployed")
}
