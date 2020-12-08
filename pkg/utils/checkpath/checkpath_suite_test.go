package checkpath_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestCheckpath(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Checkpath Suite")
}
