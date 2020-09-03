package common

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("JobReference", func(){
	var jr *JobReference

	BeforeEach(func(){
		jr = &JobReference{}

		jr.Succeeded = 0
		jr.BackoffLimit = 0
		jr.Failed = 0
		jr.Active = 0
	})

	It("should be successful", func() {
		jr.Succeeded = 1
		jr.BackoffLimit = 1

		Expect(jr.IsDone()).To(BeTrue())
		Expect(jr.IsSuccessful()).To(BeTrue())
	})

	It("should fail", func() {
		jr.Failed = 2
		jr.BackoffLimit = 1

		Expect(jr.IsDone()).To(BeTrue())
		Expect(jr.IsFailed()).To(BeTrue())
	})

	It("should fail but not be done", func() {
		jr.Failed = 1
		jr.BackoffLimit = 1

		Expect(jr.IsDone()).To(BeFalse())
		Expect(jr.IsFailed()).To(BeFalse())
	})

	It("should show active", func() {
		jr.Active = 1
		jr.BackoffLimit = 1

		Expect(jr.IsDone()).To(BeFalse())
		Expect(jr.IsActive()).To(BeTrue())
	})
})
