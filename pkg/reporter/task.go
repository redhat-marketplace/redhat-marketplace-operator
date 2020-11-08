// Copyright 2020 IBM Corp.
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
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"emperror.dev/errors"
	"github.com/google/uuid"
	"github.com/gotidy/ptr"
	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/common/log"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/managers"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Task struct {
	ReportName ReportName

	CC        ClientCommandRunner
	Cache     cache.Cache
	K8SClient client.Client
	Ctx       context.Context
	Config    *Config
	K8SScheme *runtime.Scheme
	Uploader
}

func (r *Task) Run() error {
	logger.Info("task run start")
	stopCh := make(chan struct{})
	defer close(stopCh)

	r.Cache.WaitForCacheSync(stopCh)

	logger.Info("creating reporter job")
	reporter, err := NewReporter(r)

	if err != nil {
		return err
	}

	logger.Info("starting collection")
	metrics, errorList, err := reporter.CollectMetrics(r.Ctx)

	if err != nil {
		logger.Error(err, "error collecting metrics")
		return err
	}

	reportID := uuid.New()

	logger.Info("writing report", "reportID", reportID)

	files, err := reporter.WriteReport(
		reportID,
		metrics)

	if err != nil {
		return errors.Wrap(err, "error writing report")
	}

	dirpath := filepath.Dir(files[0])
	fileName := fmt.Sprintf("%s/../upload-%s.tar.gz", dirpath, reportID.String())
	err = TargzFolder(dirpath, fileName)

	logger.Info("tarring", "outputfile", fileName)

	if r.Config.Upload {
		err = r.Uploader.UploadFile(fileName)

		if err != nil {
			return errors.Wrap(err, "error uploading file")
		}

		logger.Info("uploaded metrics", "metrics", len(metrics))
	}

	report := &marketplacev1alpha1.MeterReport{}
	err = utils.Retry(func() error {
		result, _ := r.CC.Do(
			r.Ctx,
			HandleResult(
				GetAction(types.NamespacedName(r.ReportName), report),
				OnContinue(Call(func() (ClientAction, error) {
					report.Status.MetricUploadCount = ptr.Int(len(metrics))

					report.Status.QueryErrorList = []string{}

					for _, err := range errorList {
						report.Status.QueryErrorList = append(report.Status.QueryErrorList, err.Error())
					}

					return UpdateAction(report), nil
				})),
			),
		)

		if result.Is(Error) {
			return result
		}

		return nil
	}, 3)

	if err != nil {
		log.Error(err, "failed to update report")
	}

	return nil
}

func provideApiClient(
	report *marketplacev1alpha1.MeterReport,
	promService *corev1.Service,
	config *Config,
) (api.Client, error) {

	if config.Local {
		client, err := api.NewClient(api.Config{
			Address: "http://localhost:9090",
		})

		if err != nil {
			return nil, err
		}

		return client, nil
	}

	var port int32
	name := promService.Name
	namespace := promService.Namespace
	targetPort := report.Spec.PrometheusService.TargetPort

	switch {
	case targetPort.Type == intstr.Int:
		port = targetPort.IntVal
	default:
		for _, p := range promService.Spec.Ports {
			if p.Name == targetPort.StrVal {
				port = p.Port
			}
		}
	}

	var auth = ""
	if config.TokenFile != "" {
		content, err := ioutil.ReadFile(config.TokenFile)
		if err != nil {
			return nil, err
		}
		auth = fmt.Sprintf(string(content))
	}

	conf, err := NewSecureClient(&PrometheusSecureClientConfig{
		Address:        fmt.Sprintf("https://%s.%s.svc:%v", name, namespace, port),
		ServerCertFile: config.CaFile,
		Token:          auth,
	})

	if err != nil {
		return nil, err
	}

	return conf, nil
}

func ProvideApiClient(
	// promTargetPort targetPort,
	promService *corev1.Service,
	caCert *[]byte,
) (api.Client, error) {

	var port int32
	name := promService.Name
	namespace := promService.Namespace
	fmt.Println("PROMSERVICE NAME",name)
	fmt.Println("PROMSERVICE NAMESPACE",namespace)
	targetPort := intstr.FromString("rbac")
	
	switch {
	case targetPort.Type == intstr.Int:
		port = targetPort.IntVal
	default:
		for _, p := range promService.Spec.Ports {
			if p.Name == targetPort.StrVal {
				port = p.Port
			}
		}
	}

	fmt.Println("PORT",port)

	var auth = ""

	content, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	fmt.Println("CONTENT",content)
	if err != nil {
		fmt.Println("TOKEN ERR",err)
		return nil, err
	}
	auth = fmt.Sprintf(string(content))
	fmt.Println("AUTH",auth)
	conf, err := NewSecureClientFromConfigMap(&PrometheusSecureClientConfig{
		Address:        fmt.Sprintf("https://%s.%s.svc:%v", name, namespace, port),
		// ServerCertFile:  "MIIDUTCCAjmgAwIBAgIIe5Ow8170mucwDQYJKoZIhvcNAQELBQAwNjE0MDIGA1UEAwwrb3BlbnNoaWZ0LXNlcnZpY2Utc2VydmluZy1zaWduZXJAMTU5Nzk5NjA2MDAeFw0yMDA4MjEwNzQ3MzlaFw0yMjEwMjAwNzQ3NDBaMDYxNDAyBgNVBAMMK29wZW5zaGlmdC1zZXJ2aWNlLXNlcnZpbmctc2lnbmVyQDE1OTc5OTYwNjAwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCsMtPOFBWqC4PwMf01kIcLCRdCfM/wCIJ1vvnzZYD1WspJ67rkKZzMU1TQ2T6iuTekvbw/JrrZ6QuCYjQZc4KpB3wyGv725OaPw9LCVG/GlpUtZYARjrxnMKiBtuHC+iQcL+SWEpeLHk/WSpYPclyKpFLXZunZN6sN7wTGuiDPF7pu+pw/KTtcgeun/92rS8JAKH48Y3X2248LOxmZ+niDgi91NvqTqvYey+mvdDquptE3hDHXa7PlZMW2ryIH23Mw52j+pzSKfzHBa6mstGxx9nBpLLQi8+M8pyEIfhUHMcIpQYRO5FO42fzrm3TJw6STOZecsGBci6N4W6iBJkeBAgMBAAGjYzBhMA4GA1UdDwEB/wQEAwICpDAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBSANhk2c9D9oCif2wgxY4J4oSReaTAfBgNVHSMEGDAWgBSANhk2c9D9oCif2wgxY4J4oSReaTANBgkqhkiG9w0BAQsFAAOCAQEAMMle/y6QMeFek9ZuggdtGzUK9jpoM9C83GLLOt8dpqYR2Xy4qdjpkHZRKUy2YnBOb2ulKVD4Dh7FcfdUO4AEO6J6DXBgioDwrD5FjXxLjgaq3JqX+qqFIL4yMKpTy6hxBOisfwdcX60MM8O0xVU5GOhOKKhorq4zR7cgX9Xnjo1oBmqXuFWdEUS9iecDJkTbpjDWj9gcsFjiYsefuzmm93V3w4/Y0G2M0/czvrFgbg/peqPRufEUu5gtcUupMCe/gSgFAUBwn/cuOSFhvupW6ONGTpownxQbSJ9g/dzBnX3S4niF3Lf5Qfa2d7rvkwBkVpfbwh1ATWbIK2McjQspZw==",
		Token:          auth,
		CaCert: caCert,
	})

	if conf == nil {
		fmt.Println("*--------- CONF IS NIL ----------*")
	}

	if err != nil {
		log.Error(err,"failed to setup NewSecureClient()")
		return nil, err
	}

	fmt.Println("conf",conf)

	return conf, nil
}

func getClientOptions() managers.ClientOptions {
	return managers.ClientOptions{
		Namespace:    "",
		DryRunClient: false,
	}
}

func getMarketplaceConfig(
	ctx context.Context,
	cc ClientCommandRunner,
) (config *marketplacev1alpha1.MarketplaceConfig, returnErr error) {
	config = &marketplacev1alpha1.MarketplaceConfig{}

	if result, _ := cc.Do(ctx,
		GetAction(
			types.NamespacedName{Namespace: "openshift-redhat-marketplace", Name: utils.MARKETPLACECONFIG_NAME}, config,
		)); !result.Is(Continue) {
		returnErr = errors.Wrap(result, "failed to get mkplc config")
	}

	logger.Info("retrieved meter report")
	return
}

func getMarketplaceReport(
	ctx context.Context,
	cc ClientCommandRunner,
	reportName ReportName,
) (report *marketplacev1alpha1.MeterReport, returnErr error) {
	report = &marketplacev1alpha1.MeterReport{}

	if result, _ := cc.Do(ctx, GetAction(types.NamespacedName(reportName), report)); !result.Is(Continue) {
		returnErr = errors.Wrap(result, "failed to get report")
	}

	logger.Info("retrieved meter report")
	return
}

func getPrometheusService(
	ctx context.Context,
	report *marketplacev1alpha1.MeterReport,
	cc ClientCommandRunner,
) (service *corev1.Service, returnErr error) {
	service = &corev1.Service{}

	if report.Spec.PrometheusService == nil {
		returnErr = errors.New("cannot retrieve service as the report doesn't have a value for it")
		return
	}

	name := types.NamespacedName{
		Name:      report.Spec.PrometheusService.Name,
		Namespace: report.Spec.PrometheusService.Namespace,
	}

	if result, _ := cc.Do(ctx, GetAction(name, service)); !result.Is(Continue) {
		returnErr = errors.Wrap(result, "failed to get report")
	}

	logger.Info("retrieved prometheus service")
	return
}

func QueryForPrometheusService(
	ctx context.Context,
	cc ClientCommandRunner,
) (service *corev1.Service, returnErr error) {
	service = &corev1.Service{}

	name := types.NamespacedName{
		Name:      "rhm-prometheus-meterbase",
		Namespace: "openshift-redhat-marketplace",
	}

	if result, _ := cc.Do(ctx, GetAction(name, service)); !result.Is(Continue) {
		returnErr = errors.Wrap(result, "failed to get report")
	}
	
	logger.Info("retrieved prometheus service")
	return service,nil
}

func getMeterDefinitions(
	ctx context.Context,
	report *marketplacev1alpha1.MeterReport,
	cc ClientCommandRunner,
) ([]marketplacev1alpha1.MeterDefinition, error) {
	defs := &marketplacev1alpha1.MeterDefinitionList{}

	if len(report.Spec.MeterDefinitions) > 0 {
		return report.Spec.MeterDefinitions, nil
	}

	result, _ := cc.Do(ctx,
		HandleResult(
			ListAction(defs, client.InNamespace("")),
			OnContinue(Call(func() (ClientAction, error) {
				for _, item := range defs.Items {
					item.Status = marketplacev1alpha1.MeterDefinitionStatus{}
				}

				report.Spec.MeterDefinitions = defs.Items

				return UpdateAction(report), nil
			})),
		),
	)

	if result.Is(Error) {
		return nil, errors.Wrap(result, "failed to get meterdefs")
	}

	if result.Is(NotFound) {
		return []marketplacev1alpha1.MeterDefinition{}, nil
	}

	return defs.Items, nil
}
