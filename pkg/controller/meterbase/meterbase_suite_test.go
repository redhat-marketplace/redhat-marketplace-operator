package meterbase_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestMeterbase(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Meterbase Suite")
}
