package reporter_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	loggerf "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/logger"
)

func TestReporter(t *testing.T) {
	loggerf.SetLoggerToDevelopmentZap()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Reporter Suite")
}
