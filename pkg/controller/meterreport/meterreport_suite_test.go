package meterreport_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestMeterreport(t *testing.T) {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))
	RegisterFailHandler(Fail)
	RunSpecs(t, "Meterreport Suite")
}
