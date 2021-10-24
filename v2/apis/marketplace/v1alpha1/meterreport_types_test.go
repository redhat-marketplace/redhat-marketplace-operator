package v1alpha1

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("meterreport_types", func() {

	Context("uploadDetails", func() {
		var (
			details UploadDetailConditions
		)

		BeforeEach(func() {
			details = UploadDetailConditions{}
		})

		It("should provide helper functions", func() {
			details.Set(UploadDetails{
				Target: "foo",
				Status: UploadStatusSuccess,
			})
			Expect(details).To(HaveLen(1))
			Expect(details.Get("foo")).ToNot(BeNil())
			Expect(details.Get("foo").ID).ToNot(Equal("foo"))
			Expect(details.AllSuccesses()).To(BeTrue())

			details.Set(UploadDetails{
				Target: "bar",
				Status: UploadStatusFailure,
				Error:  "ouch",
			})

			Expect(details).To(HaveLen(2))
			Expect(details.Get("foo")).ToNot(BeNil())
			Expect(details.Get("foo").ID).ToNot(Equal("foo"))
			Expect(details.AllSuccesses()).To(BeFalse())
			Expect(details.OneSucessOf([]string{"foo"})).To(BeTrue())

			details.Append(UploadDetailConditions{
				{
					Target: "baz",
					Status: UploadStatusSuccess,
				},
				{
					Target: "bar",
					Status: UploadStatusSuccess,
				},
			})

			Expect(details).To(HaveLen(3))
			Expect(details.Get("bar")).ToNot(BeNil())
			Expect(details.AllSuccesses()).To(BeTrue())

			Expect(details.OneSucessOf([]string{"doesn'texist"})).To(BeFalse())
		})

		It("should handle no objects", func() {
			Expect(details).To(HaveLen(0))
			Expect(details.Get("foo")).To(BeNil())
			Expect(details.AllSuccesses()).To(BeFalse())
			Expect(details.OneSucessOf([]string{"foo"})).To(BeFalse())
		})

		It("should handle nil", func() {
			details = nil

			Expect(details).To(HaveLen(0))
			Expect(details.Get("foo")).To(BeNil())
			Expect(details.AllSuccesses()).To(BeFalse())
			Expect(details.OneSucessOf([]string{"foo"})).To(BeFalse())

			details.Set(UploadDetails{
				Target: "bar",
				Status: UploadStatusFailure,
				Error:  "ouch",
			})

			Expect(details).To(HaveLen(1))

			details = nil

			details.Append(UploadDetailConditions{
				{
					Target: "bar",
					Status: UploadStatusFailure,
					Error:  "ouch",
				},
			})

			Expect(details).To(HaveLen(1))
		})

		It("should collect errors", func() {
			details.Append(UploadDetailConditions{
				{
					Target: "bar",
					Status: UploadStatusFailure,
					Error:  "ouch",
				},
				{
					Target: "baz",
					Status: UploadStatusFailure,
					Error:  "ouch",
				},
			})

			Expect(details.Errors()).To(HaveOccurred())
		})
	})
})
