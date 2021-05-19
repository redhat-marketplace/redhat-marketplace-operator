package marketplace

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = FDescribe("report time", func() {

	It("should execute at the right time", func() {
		now := time.Now()
		when := now.Add(5 * time.Minute)
		Expect(waitTime(now, when, 0)).To(BeNumerically("~", 5*time.Minute))
	})

	It("should execute at the right time", func() {
		now := time.Now()
		when := now.Add(10 * time.Minute)
		Expect(waitTime(now, when, 2)).To(BeNumerically("~", 12*time.Minute))
	})

	It("should execute at the right time", func() {
		now := time.Now()
		when := now.Add(-5 * time.Minute)
		Expect(waitTime(now, when, 0)).To(BeNumerically("~", 0*time.Minute))
	})

	It("should execute at the right time", func() {
		now := time.Now()
		when := now.Add(-5 * time.Minute)
		fmt.Println(when)
		Expect(waitTime(now, when, 5)).To(BeNumerically("~", 0*time.Minute))
	})
})
