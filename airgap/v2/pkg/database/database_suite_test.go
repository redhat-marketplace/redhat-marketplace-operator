package database

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestDatabase(t *testing.T) {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter)))
	RegisterFailHandler(Fail)
	RunSpecs(t, "Database Suite")
}
