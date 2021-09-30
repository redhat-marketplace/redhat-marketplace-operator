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

package reconciler

import (
	"context"
	"os"
	"time"

	"emperror.dev/errors"

	"github.com/gotidy/ptr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter"
	"github.com/spf13/cobra"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("reconcile")

var namespace, cafile, tokenFile string
var localFilePath, deployedNamespace string
var dataServiceTokenFile, dataServiceCertFile string
var prometheusService, prometheusNamespace, prometheusPort string
var reporterSchema string
var uploadTargets []string
var local, upload bool
var retry int

var ReconcileCmd = &cobra.Command{
	Use:   "reconcile",
	Short: "Runs and uploads reports",
	Run: func(cmd *cobra.Command, args []string) {
		if namespace == "" {
			log.Error(errors.New("namespace not provided"), "namespace not provided")
			os.Exit(1)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		tmpDir := os.TempDir()

		targets := reporter.UploaderTargets{}
		for _, uploadTarget := range uploadTargets {
			uploadTarget := reporter.MustParseUploaderTarget(uploadTarget)
			log.Info("upload target", "target set to", uploadTarget.Name())

			switch v := uploadTarget.(type) {
			case *reporter.LocalFilePathUploader:
				v.LocalFilePath = localFilePath
			}
			targets = append(targets, uploadTarget)
		}

		cfg := &reporter.Config{
			OutputDirectory:      tmpDir,
			Retry:                ptr.Int(retry),
			CaFile:               cafile,
			TokenFile:            tokenFile,
			DataServiceTokenFile: dataServiceTokenFile,
			DataServiceCertFile:  dataServiceCertFile,
			Local:                local,
			Upload:               upload,
			UploaderTargets:      targets,
			DeployedNamespace:    deployedNamespace,
			PrometheusService:    prometheusService,
			PrometheusNamespace:  prometheusNamespace,
			PrometheusPort:       prometheusPort,
			ReporterSchema:       reporterSchema,
		}
		cfg.SetDefaults()

		recorder, err := reporter.NewEventRecorder(ctx, cfg)
		if err != nil {
			log.Error(err, "couldn't initialize event recorder")
			os.
				Exit(1)
		}

		job := getJob()

		task, err := reporter.NewReconcileTask(
			ctx,
			cfg,
			reporter.Namespace(namespace),
		)

		if err != nil {
			var comp *reporter.ReportJobError

			if errors.As(err, &comp) {
				log.Error(err, "report job error")
				recorder.Event(job, corev1.EventTypeWarning, "ReportJobError", "No insights")
			}
			log.Error(err, "couldn't initialize task")
			os.Exit(1)
		}

		err = task.Run(ctx)
		if err != nil {
			var comp *reporter.ReportJobError

			if errors.As(err, &comp) {
				log.Error(err, "report job error")
				recorder.Event(job, corev1.EventTypeWarning, "ReportJobError", "No insights")
			}

			log.Error(err, "error running task")
			os.Exit(1)
		}

		os.Exit(0)
	},
}

func init() {
	ReconcileCmd.Flags().StringVar(&namespace, "namespace", "", "namespace of the report")

	ReconcileCmd.Flags().StringVar(&cafile, "cafile", "", "cafile for prometheus")
	ReconcileCmd.Flags().StringVar(&tokenFile, "tokenfile", "/var/run/secrets/kubernetes.io/serviceaccount/token", "token file for prometheus")

	ReconcileCmd.Flags().StringVar(&dataServiceTokenFile, "dataServiceTokenFile", "", "token file for the data service")
	ReconcileCmd.Flags().StringVar(&dataServiceCertFile, "dataServiceCertFile", "", "cert file for the data service")

	ReconcileCmd.Flags().StringSliceVar(&uploadTargets, "uploadTargets", []string{"redhat-insights"}, "comma seperated list of targets to upload to")
	ReconcileCmd.Flags().StringVar(&localFilePath, "localFilePath", ".", "target to upload to")
	ReconcileCmd.Flags().BoolVar(&local, "local", false, "run locally")
	ReconcileCmd.Flags().BoolVar(&upload, "upload", true, "to upload the payload")
	ReconcileCmd.Flags().IntVar(&retry, "retry", 3, "number of retries")
	ReconcileCmd.Flags().StringVar(&deployedNamespace, "deployedNamespace", "openshift-redhat-marketplace", "namespace where the rhm operator is deployed")

	ReconcileCmd.Flags().StringVar(&prometheusService, "prometheus-service", "rhm-prometheus-meterbase", "token file for the data service")
	ReconcileCmd.Flags().StringVar(&prometheusNamespace, "prometheus-namespace", "openshift-redhat-marketplace", "cert file for the data service")
	ReconcileCmd.Flags().StringVar(&prometheusPort, "prometheus-port", "rbac", "cert file for the data service")

	ReconcileCmd.Flags().StringVar(&reporterSchema, "reporterSchema", "v1alpha1", "reporter version schema to write")
}

func getJob() *batchv1.Job {

	cl, err := client.New(config.GetConfigOrDie(), client.Options{})
	if err != nil {
		log.Error(err, "failed to create client")
		os.Exit(1)
	}

	job := &batchv1.Job{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: os.Getenv("JOB_NAME"), Namespace: os.Getenv("POD_NAMESPACE")}, job)
	if err != nil {
		log.Error(err, "could not get running job")
		os.Exit(1)
	}

	return job
}
