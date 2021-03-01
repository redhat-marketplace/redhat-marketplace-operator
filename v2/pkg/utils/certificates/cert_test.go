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

package utils

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/cloudflare/cfssl/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("CertIssuer", func() {
	var (
		certIssuer     *CertIssuer
		err            error
		namespacedName = types.NamespacedName{
			Name:      "test",
			Namespace: "test",
		}
	)

	BeforeEach(func() {
		log := logf.Log.WithName("certissuer_test")
		certIssuer, err = NewCertIssuer(log)
		Expect(err).To(Succeed(), "unable to create cert issuer")
	})

	It("has generated certificate authority", func() {
		Expect(certIssuer.ca).ToNot(BeNil())
	})

	It("generates signed certificate", func() {
		pub, key, err := certIssuer.CreateCertFromCA(namespacedName)
		Expect(err).To(Succeed(), "failed to generate certificate")
		Expect(len(pub)).Should(BeNumerically(">", 0))
		Expect(len(key)).Should(BeNumerically(">", 0))
		Expect(certIssuer.ca).ToNot(BeNil())
	})

	It("validates signed certificate", func() {
		pub, key, err := certIssuer.CreateCertFromCA(namespacedName)
		Expect(err).To(Succeed(), "failed to generate certificate")

		_, err = helpers.ParseCertificatePEM(pub)
		Expect(err).To(Succeed(), "failed to verify client cert")

		_, err = helpers.ParsePrivateKeyPEM(key)
		Expect(err).To(Succeed(), "failed to verify client cert")

		roots := x509.NewCertPool()
		ok := roots.AppendCertsFromPEM(certIssuer.CAPublicKey())
		Expect(ok).To(BeTrue())

		block, _ := pem.Decode(pub)
		Expect(block).ToNot(BeNil())

		cert, err := x509.ParseCertificate(block.Bytes)
		Expect(err).To(Succeed(), "failed to parse certificate")

		opts := x509.VerifyOptions{
			DNSName: fmt.Sprintf("%s.%s.svc", namespacedName.Name, namespacedName.Namespace),
			Roots:   roots,
		}
		_, err = cert.Verify(opts)
		Expect(err).To(Succeed(), "failed to verify signed cert")
	})
})
