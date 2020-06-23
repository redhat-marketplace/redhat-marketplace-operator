package reconcileutils_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestReconcileutils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Reconcileutils Suite")
}
