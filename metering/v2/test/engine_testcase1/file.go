package testcase1

import (
	"embed"

	"github.com/redhat-marketplace/redhat-marketplace-operator/metering/v2/test"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)


var (

	//go:embed *
	fs embed.FS

	testCase1 test.UnstructuredFS = test.UnstructuredFS{
		FS: fs,
	}

	MdefChargeBack *unstructured.Unstructured = testCase1.MustGetUnstructured("mdef-chargeback.yaml")
	CSVLicensing *unstructured.Unstructured = testCase1.MustGetUnstructured("csv-licensing-service.yaml")
	ServiceInstance *unstructured.Unstructured = testCase1.MustGetUnstructured("service-ibm-licensing-service-instance.yaml")
	ServicePrometheus *unstructured.Unstructured = testCase1.MustGetUnstructured("service-ibm-licensing-service-prometheus.yaml")
)
