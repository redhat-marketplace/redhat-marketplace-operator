// Copyright 2021 Google LLC
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

package common

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("JobReference", func() {
	var jr *JobReference

	BeforeEach(func() {
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
