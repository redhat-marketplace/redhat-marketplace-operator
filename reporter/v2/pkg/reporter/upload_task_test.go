// Copyright 2021 IBM Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
