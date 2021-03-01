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
	"fmt"
	"time"

	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/helpers"
	"github.com/cloudflare/cfssl/initca"
	"github.com/cloudflare/cfssl/signer"
	"github.com/cloudflare/cfssl/signer/local"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
)

type CertIssuer struct {
	ca     CertificateAuthority
	logger logr.Logger
}

type CertIssuerConfig struct {
	Namespace string
	RetryTime time.Duration
}

type CertificateAuthority struct {
	PublicKey  []byte
	PrivateKey []byte
}

func NewCertIssuer(
	l logr.Logger,
) (*CertIssuer, error) {
	cert, key, err := createCertificateAuthority()
	if err != nil {
		l.Error(err, "Unable to create Certificate Authority")
		return nil, err
	}

	return &CertIssuer{
		ca: CertificateAuthority{
			PublicKey:  cert,
			PrivateKey: key,
		},
		logger: l,
	}, nil
}

// createCertificateAuthority creates CA for self signed certificates
func createCertificateAuthority() ([]byte, []byte, error) {
	req := csr.CertificateRequest{
		KeyRequest: &csr.KeyRequest{
			A: "rsa",
			S: 2048,
		},
		CN: "rhmp_ca",
		Hosts: []string{
			"rhmp_ca",
		},
		CA: &csr.CAConfig{
			Expiry: "8760h",
		},
	}

	cert, _, key, err := initca.New(&req)
	if err != nil {
		return nil, nil, err
	}

	return cert, key, nil
}

// CreateCertFromCA generates certs signed with CA keys
func (ci *CertIssuer) CreateCertFromCA(
	namespacedName types.NamespacedName,
) ([]byte, []byte, error) {
	parsedCaCert, err := helpers.ParseCertificatePEM(ci.ca.PublicKey)
	if err != nil {
		return nil, nil, err
	}
	parsedCaKey, err := helpers.ParsePrivateKeyPEM(ci.ca.PrivateKey)
	if err != nil {
		return nil, nil, err
	}

	svcFullname := fmt.Sprintf("%s.%s.svc", namespacedName.Name, namespacedName.Namespace)
	req := csr.CertificateRequest{
		KeyRequest: &csr.KeyRequest{
			A: "rsa",
			S: 2048,
		},
		CN: svcFullname,
		Hosts: []string{
			svcFullname,
			svcFullname + ".cluster",
			svcFullname + ".cluster.local",
		},
	}
	certReq, key, err := csr.ParseRequest(&req)
	if err != nil {
		return nil, nil, err
	}

	csigner, err := local.NewSigner(parsedCaKey, parsedCaCert, signer.DefaultSigAlgo(parsedCaKey), nil)
	if err != nil {
		return nil, nil, err
	}

	signedCert, err := csigner.Sign(signer.SignRequest{
		Hosts: []string{
			svcFullname,
			svcFullname + ".cluster",
			svcFullname + ".cluster.local",
		},
		Request: string(certReq),
		Subject: &signer.Subject{
			CN: svcFullname,
		},
		Profile: svcFullname,
	})
	if err != nil {
		return nil, nil, err
	}

	return signedCert, key, nil
}

func (ci *CertIssuer) CAPublicKey() []byte {
	return ci.ca.PublicKey
}
