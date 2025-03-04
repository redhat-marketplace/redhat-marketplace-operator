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
	"strconv"
	"time"

	"emperror.dev/errors"

	"github.com/gotidy/ptr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/reporter"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/uploaders"
	"github.com/spf13/cobra"
	k8sapiflag "k8s.io/component-base/cli/flag"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("reconcile")

var namespace, cafile, tokenFile string
var localFilePath, deployedNamespace string
var dataServiceTokenFile, dataServiceCertFile string
var prometheusService, prometheusNamespace, prometheusPort string
var reporterSchema string
var isDisconnected string
var uploadTargets []string
var local, upload bool
var retry int
var minVersion string
var cipherSuites []string

var ReconcileCmd = &cobra.Command{
	Use:   "reconcile",
	Short: "Runs and uploads reports",
	RunE: func(cmd *cobra.Command, args []string) error {
		if namespace == "" {
			return errors.New("namespace not provided")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
		defer cancel()

		tmpDir := os.TempDir()

		targets := uploaders.UploaderTargets{}
		for _, uploadTarget := range uploadTargets {
			uploadTarget := uploaders.MustParseUploaderTarget(uploadTarget)
			log.Info("upload target", "target set to", uploadTarget.Name())

			switch v := uploadTarget.(type) {
			case *uploaders.LocalFilePathUploader:
				v.LocalFilePath = localFilePath
			}
			targets = append(targets, uploadTarget)
		}

		isDisconnectedBool, err := strconv.ParseBool(isDisconnected)
		if err != nil {
			return errors.Wrap(err, "error converting IS_DISCONNECTED to bool")
		}

		tlsVersion, err := k8sapiflag.TLSVersion(minVersion)
		if err != nil {
			return errors.Wrap(err, "TLS version invalid")
		}

		tlsCipherSuites, err := k8sapiflag.TLSCipherSuites(cipherSuites)
		if err != nil {
			return errors.Wrap(err, "failed to convert TLS cipher suite name to ID")
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
			IsDisconnected:       isDisconnectedBool,
			UploaderTargets:      targets,
			DeployedNamespace:    deployedNamespace,
			PrometheusService:    prometheusService,
			PrometheusNamespace:  prometheusNamespace,
			PrometheusPort:       prometheusPort,
			ReporterSchema:       reporterSchema,
			MinVersion:           tlsVersion,
			CipherSuites:         tlsCipherSuites,
		}
		err = cfg.SetDefaults()
		if err != nil {
			return errors.Wrap(err, "couldn't get defaults")
		}

		broadcaster, stopBroadcast, err := reporter.NewEventBroadcaster(cfg)
		if err != nil {
			return errors.Wrap(err, "couldn't initialize event broadcaster")
		}
		defer stopBroadcast()

		task, err := reporter.NewReconcileTask(
			ctx,
			cfg,
			broadcaster,
			reporter.Namespace(namespace),
			reporter.NewTask,
			reporter.NewUploadTask,
		)

		if err != nil {
			return errors.Wrap(err, "couldn't initialize task")
		}

		err = task.Run(ctx)
		if err != nil {
			return errors.Wrap(err, "error running task")
		}

		return nil
	},
}

func init() {
	ReconcileCmd.Flags().StringVar(&namespace, "namespace", "", "namespace of the report")

	ReconcileCmd.Flags().StringVar(&cafile, "cafile", "", "cafile for prometheus")
	ReconcileCmd.Flags().StringVar(&tokenFile, "tokenfile", "/var/run/secrets/kubernetes.io/serviceaccount/token", "token file for prometheus")

	ReconcileCmd.Flags().StringVar(&dataServiceTokenFile, "dataServiceTokenFile", "", "token file for the data service")
	ReconcileCmd.Flags().StringVar(&dataServiceCertFile, "dataServiceCertFile", "", "cert file for the data service")

	ReconcileCmd.Flags().StringSliceVar(&uploadTargets, "uploadTargets", []string{"redhat-marketplace"}, "comma separated list of targets to upload to")
	ReconcileCmd.Flags().StringVar(&localFilePath, "localFilePath", ".", "target to upload to")
	ReconcileCmd.Flags().BoolVar(&local, "local", false, "run locally")
	ReconcileCmd.Flags().BoolVar(&upload, "upload", true, "to upload the payload")
	ReconcileCmd.Flags().StringVar(&isDisconnected, "isDisconnected", os.Getenv("IS_DISCONNECTED"), "is the reporter running in a disconnected environment")
	ReconcileCmd.Flags().IntVar(&retry, "retry", 24, "number of retries")
	ReconcileCmd.Flags().StringVar(&deployedNamespace, "deployedNamespace", os.Getenv("POD_NAMESPACE"), "namespace where the rhm operator is deployed")

	ReconcileCmd.Flags().StringVar(&prometheusService, "prometheus-service", "rhm-prometheus-meterbase", "token file for the data service")
	ReconcileCmd.Flags().StringVar(&prometheusNamespace, "prometheus-namespace", "openshift-redhat-marketplace", "cert file for the data service")
	ReconcileCmd.Flags().StringVar(&prometheusPort, "prometheus-port", "rbac", "cert file for the data service")

	ReconcileCmd.Flags().StringVar(&reporterSchema, "reporterSchema", "v3alpha1", "reporter version schema to write")

	ReconcileCmd.Flags().StringVar(&minVersion, "tls-min-version", "VersionTLS12", "Minimum TLS version supported. Value must match version names from https://golang.org/pkg/crypto/tls/#pkg-constants.")
	ReconcileCmd.Flags().StringSliceVar(&cipherSuites,
		"tls-cipher-suites",
		[]string{"TLS_AES_128_GCM_SHA256",
			"TLS_AES_256_GCM_SHA384",
			"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
			"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"},
		"Comma-separated list of cipher suites for the server. Values are from tls package constants (https://golang.org/pkg/crypto/tls/#pkg-constants). If omitted, a subset will be used")

}
