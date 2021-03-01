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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	utils "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/certificates"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("CertIssuerController", func() {
	Describe("check certificate issuer CA", func() {
		var (
			ctrl *CertIssuerReconciler
		)

		BeforeEach(func() {
			var (
				log = logf.Log.WithName("certissuer_controller")
			)

			ci, err := utils.NewCertIssuer(log)
			Expect(err).To(Succeed())
			ctrl = &CertIssuerReconciler{
				certIssuer: ci,
			}
		})

		It("should have CA generated", func() {
			pk := ctrl.certIssuer.CAPublicKey()
			Expect(pk).ToNot(BeNil())
		})
	})
})
