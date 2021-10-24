package reporter

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/uploaders"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
)

var _ = Describe("upload_task", func() {
	Context("findStatus", func() {
		var (
			statuses marketplacev1alpha1.UploadDetailConditions
		)

		It("should report success if required statuses are present", func() {
			statuses = marketplacev1alpha1.UploadDetailConditions{
				{
					Target: uploaders.UploaderTargetMarketplace.Name(),
					Status: marketplacev1alpha1.UploadStatusSuccess,
				},
			}

			success, condition := findStatus(statuses)
			Expect(success).To(BeTrue())
			Expect(condition.IsTrue()).To(BeTrue())

			statuses = marketplacev1alpha1.UploadDetailConditions{
				{
					Target: uploaders.UploaderTargetMarketplace.Name(),
					Status: marketplacev1alpha1.UploadStatusFailure,
					Error:  "oh no",
				},
			}

			success, condition = findStatus(statuses)
			Expect(success).To(BeFalse())
			Expect(condition.IsTrue()).To(BeFalse())
		})
	})
})
