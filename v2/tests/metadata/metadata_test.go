// Copyright 2020 IBM Corp.
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

package metadata

import (
	"io/ioutil"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Metadata", func() {
	BeforeSuite(func() {
		command := exec.Command("make", "clean", "compile-helm")
		command.Stderr = GinkgoWriter
		command.Stdout = GinkgoWriter
		command.Dir = "../.."
		err := command.Run()
		out, _ := command.CombinedOutput()
		Expect(err).ShouldNot(HaveOccurred(), string(out))
	})

	AssertGoldenFileMatch := func(originalFile, goldenFile string) func() {
		return func() {
			Skip("Needs rework")
			dat, err := ioutil.ReadFile(originalFile)
			Expect(err).ShouldNot(HaveOccurred())

			goldenDat, err := ioutil.ReadFile(goldenFile)
			Expect(err).ShouldNot(HaveOccurred())

			datArray := strings.Split(string(dat), "---")
			goldenArray := strings.Split(string(goldenDat), "---")

			Expect(datArray).Should(HaveLen(len(goldenArray)))

			for i := range datArray {
				Expect(datArray[i]).Should(MatchYAML(goldenArray[i]))
			}
		}
	}

	It("should have consistent role output", AssertGoldenFileMatch("../../deploy/role.yaml", "./golden/role.yaml"))

	It("should have consistent serviceaccount output", AssertGoldenFileMatch("../../deploy/service_account.yaml", "./golden/service_account.yaml"))

	It("should have consistent role bindings output", AssertGoldenFileMatch("../../deploy/role_binding.yaml", "./golden/role_binding.yaml"))
})
