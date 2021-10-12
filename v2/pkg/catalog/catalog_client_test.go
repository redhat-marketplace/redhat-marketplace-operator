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

package catalog

import (
	"log"
	"net"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega/ghttp"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/rhmotransport"

	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"

	. "github.com/onsi/gomega"
)

//TODO: in progress
var _ = Describe("Catalog Client", func() {

	var (
		catalogClientMockServer *ghttp.Server
		communityMeterDefPath   = "/" + GetCommunityMeterdefinitionsEndpoint
	)

	const (
		timeout  = time.Second * 100
		interval = time.Second * 3
	)

	cr := &CatalogRequest{
		CSVInfo: CSVInfo{
			Name:      "memcached-operator.v0.0.1",
			Namespace: "openshift-redhat-marketplace",
			Version:   "0.0.1",
		},
		SubInfo: SubInfo{
			PackageName:   "memcached-operator-rhmp",
			CatalogSource: "test-catalog-source",
		},
	}

	BeforeEach(func() {
		customListener, err := net.Listen("tcp", listenerAddress)
		Expect(err).ToNot(HaveOccurred())

		//TODO: use NewTlsServer()*
		catalogClientMockServer = ghttp.NewUnstartedServer()
		catalogClientMockServer.HTTPTestServer.Listener.Close()
		catalogClientMockServer.HTTPTestServer.Listener = customListener
		catalogClientMockServer.SetAllowUnhandledRequests(true)

		_, serverTLSConf, _, err := certsetup()
		if err != nil {
			log.Fatal(err)
		}

		catalogClientMockServer.HTTPTestServer.TLS = serverTLSConf
		catalogClientMockServer.Start()
	})

	AfterEach(func() {
		catalogClientMockServer.Close()
	})

	Context("Set Transport", func() {
		It("Should use secure client unless overridden", func() {
			Expect(catalogClient.UseSecureClient).To(Equal(true))
		})
	})

	Context("Retry 401", func() {
		BeforeEach(func() {
			unauthorizedBody := []byte(`Unauthorized`)

			catalogClientMockServer.RouteToHandler(
				"POST", communityMeterDefPath, ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", communityMeterDefPath),
					ghttp.RespondWith(http.StatusUnauthorized, unauthorizedBody),
				))
		})

		It("Should retry on Auth errors", func() {

			_, err := catalogClient.ListMeterdefintionsFromFileServer(cr)
			utils.PrettyPrint(err.Error())
			Expect(err.Error()).To(ContainSubstring("giving up after 6 attempt(s): auth error on call to meterdefinition catalog server. Call returned with: 401 Unauthorized"))
		})
	})

	Context("Retry 500", func() {
		BeforeEach(func() {
			internalServerErrorBody := []byte(`internal server error`)

			catalogClientMockServer.RouteToHandler(
				"POST", communityMeterDefPath, ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", communityMeterDefPath),
					ghttp.RespondWith(http.StatusInternalServerError, internalServerErrorBody),
				))
		})

		It("Should retry on 500 range errors", func() {
			_, err := catalogClient.ListMeterdefintionsFromFileServer(cr)
			utils.PrettyPrintWithLog(err.Error(), "500 retry error:")
			Expect(err.Error()).To(ContainSubstring("giving up after 6 attempt(s): unexpected HTTP status 500 Internal Server Error"))
		})
	})

	Context("arbitrary error", func() {
		BeforeEach(func() {
			arbitraryErrorBody := []byte(`arbitrary error`)

			catalogClientMockServer.RouteToHandler(
				"POST", communityMeterDefPath, ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", communityMeterDefPath),
					ghttp.RespondWith(http.StatusBadRequest, arbitraryErrorBody),
				))
		})

		It("Should not retry on arbitrary errors and return the error", func() {
			_, err := catalogClient.ListMeterdefintionsFromFileServer(cr)
			Expect(err.Error()).To(ContainSubstring("Error querying file server for community meter definitions: 400 Bad Request"))
		})
	})

})

func (m *MockAuthBuilderConfig) FindAuthOffCluster() (*rhmotransport.AuthValues, error) {
	certBytes, _, _, err := certsetup()
	if err != nil {
		log.Fatal(err)
	}

	return &rhmotransport.AuthValues{
		ServiceFound: true,
		Cert:         certBytes,
		AuthToken:    "test token",
	}, nil
}

/*
	TODO: this is copied from: https://gist.github.com/shaneutt/5e1995295cff6721c89a71d13a71c251
	MIT License

	Copyright (c) 2020 Shane Utt

	Permission is hereby granted, free of charge, to any person obtaining a copy
	of this software and associated documentation files (the "Software"), to deal
	in the Software without restriction, including without limitation the rights
	to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
	copies of the Software, and to permit persons to whom the Software is
	furnished to do so, subject to the following conditions:

	The above copyright notice and this permission notice shall be included in all
	copies or substantial portions of the Software.

	THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
	IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
	FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
	AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
	LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
	OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
	SOFTWARE.
*/
func certsetup() (caBundle []byte, serverTLSConf *tls.Config, clientTLSConf *tls.Config, err error) {
	// set up our CA certificate
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			Organization:  []string{"Company, INC."},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{"Golden Gate Bridge"},
			PostalCode:    []string{"94016"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	// create our private and public key
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, nil, err
	}

	// create the CA
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil, nil, err
	}

	// pem encode
	caPEM := new(bytes.Buffer)
	pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})

	caPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	})

	// set up our server certificate
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			Organization:  []string{"Company, INC."},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{"Golden Gate Bridge"},
			PostalCode:    []string{"94016"},
		},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	certPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, nil, err
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil, nil, err
	}

	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	caBundle = certPEM.Bytes()

	certPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
	})

	serverCert, err := tls.X509KeyPair(certPEM.Bytes(), certPrivKeyPEM.Bytes())
	if err != nil {
		return nil, nil, nil, err
	}

	serverTLSConf = &tls.Config{
		Certificates: []tls.Certificate{serverCert},
	}

	var transport http.RoundTripper = &http.Transport{
		TLSClientConfig: serverTLSConf,
		Proxy:           http.ProxyFromEnvironment,
	}

	transport = rhmotransport.WithBearerAuth(transport, "test token")

	certpool := x509.NewCertPool()
	certpool.AppendCertsFromPEM(caPEM.Bytes())
	clientTLSConf = &tls.Config{
		RootCAs: certpool,
	}

	return
}
