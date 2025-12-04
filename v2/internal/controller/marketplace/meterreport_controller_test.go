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

package marketplace

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("report time", func() {

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
		Expect(waitTime(now, when, 5)).To(BeNumerically("~", 0*time.Minute))
	})
})
