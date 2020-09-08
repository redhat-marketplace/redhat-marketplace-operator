package operrors_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestOperrors(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Operrors Suite")
}
