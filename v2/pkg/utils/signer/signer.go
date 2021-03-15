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

//go:generate go-bindata -o bindata.go -prefix "../../../assets/" -pkg signer ../../../assets/signer/...

package signer

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"io"
	"io/ioutil"
	"os"

	"emperror.dev/errors"
	//"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/manifests"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const SignerCaCertificate = "signer/ca.pem"

func MustAssetReader(asset string) io.Reader {
	return bytes.NewReader(MustAsset(asset))
}

// github.com/manifestival/manifestival/internal/sources/yaml.go
func Decode(reader io.Reader) ([]unstructured.Unstructured, error) {
	decoder := yaml.NewYAMLToJSONDecoder(reader)
	objs := []unstructured.Unstructured{}
	var err error
	for {
		out := unstructured.Unstructured{}
		err = decoder.Decode(&out)
		if err == io.EOF {
			break
		}
		if err != nil || len(out.Object) == 0 {
			continue
		}
		objs = append(objs, out)
	}
	if err != io.EOF {
		return nil, err
	}
	return objs, nil
}

func PrivateKeyFromPemFile(privateKeyFile string, privateKeyPassword string) (*rsa.PrivateKey, error) {
	privPEMData, err := ioutil.ReadFile(privateKeyFile)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to read private key file")
	}

	pemblock, _ := pem.Decode(privPEMData)
	if pemblock == nil || pemblock.Type != "RSA PRIVATE KEY" {
		return nil, errors.Wrap(err, "Unable to decode private key")
	}

	var pemBlockBytes []byte
	if privateKeyPassword != "" {
		pemBlockBytes, err = x509.DecryptPEMBlock(pemblock, []byte(privateKeyPassword))
	} else {
		pemBlockBytes = pemblock.Bytes
	}

	var parsedPrivateKey interface{}
	if parsedPrivateKey, err = x509.ParsePKCS1PrivateKey(pemBlockBytes); err != nil {
		if parsedPrivateKey, err = x509.ParsePKCS8PrivateKey(pemBlockBytes); err != nil {
			return nil, errors.Wrap(err, "Unable to parse private key")
		}
	}

	var privateKey *rsa.PrivateKey
	var ok bool
	privateKey, ok = parsedPrivateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.Wrap(err, "Unable to parse private key")
	}

	return privateKey, nil
}

func CertificateFromPemFile(pemFile string) (*x509.Certificate, error) {
	pemData, err := ioutil.ReadFile(pemFile)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to read pem file")
	}

	return CertificateFromPemBytes(pemData)
}

func CertificateFromPemBytes(pemData []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, errors.New("failed to decode certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse certificate block")
	}

	pub := cert.PublicKey
	switch pub.(type) {
	case *rsa.PublicKey:
		return cert, nil
	default:
		return nil, errors.New("Certificate PublicKey is not RSA. Only RSA is supported.")
	}
}

func CertificateFromAssets() (*x509.Certificate, error) {
	caCertReader := MustAssetReader(SignerCaCertificate)
	pemData, err := ioutil.ReadAll(caCertReader)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to read pem file")
	}

	return CertificateFromPemBytes(pemData)
}

// This should be the bytes we sign, or verify signature on
func UnstructuredToGVKSpecBytes(uobj unstructured.Unstructured) ([]byte, error) {
	gvkspecuobj := unstructured.Unstructured{}
	gvkspecuobj.SetGroupVersionKind(uobj.GroupVersionKind())
	gvkspecuobj.Object["spec"] = uobj.Object["spec"]

	bytes, err := gvkspecuobj.MarshalJSON()
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

func IsInputFromPipe() bool {
	info, _ := os.Stdin.Stat()
	return info.Mode()&os.ModeCharDevice == 0
}

func OpenInputFile(in string) (io.ReadCloser, error) {
	info, err := os.Stat(in)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		return nil, errors.New("input file is directory, not file")
	}

	file, err := os.Open(in)
	if err != nil {
		return nil, err
	}

	return file, nil
}

func VerifyCert(root, signed *x509.Certificate) error {
	roots := x509.NewCertPool()
	roots.AddCert(root)
	vopts := x509.VerifyOptions{
		Roots: roots,
	}

	if _, err := signed.Verify(vopts); err != nil {
		return err
	}
	return nil
}

func VerifySignatureArray(uobjs []unstructured.Unstructured, caCert *x509.Certificate) error {
	for _, uobj := range uobjs {
		return VerifySignature(uobj, caCert)
	}
	return nil
}

func VerifySignature(uobj unstructured.Unstructured, caCert *x509.Certificate) error {
	//The Unstructured object could be an UnstructuredList
	if uobj.IsList() {
		uobjList, err := uobj.ToList()
		if err != nil {
			return err
		}
		return VerifySignatureArray(uobjList.Items, caCert)
	} else {
		annotations := uobj.GetAnnotations()

		// verify pubCert against caCert

		pubKeyBytes := []byte(annotations["marketplace.redhat.com/publickey"])

		pubCert, err := CertificateFromPemBytes(pubKeyBytes)
		if err != nil {
			return errors.Wrap(err, "public key annotation is malformed")
		}

		VerifyCert(caCert, pubCert)
		if err != nil {
			return errors.Wrap(err, "failed to verify public certificate against ca certificate")
		}

		// verify content & signature

		// Reduce Object to GVK+Spec
		bytes, err := UnstructuredToGVKSpecBytes(uobj)
		if err != nil {
			return errors.Wrap(err, "could not MarshalJSON")
		}
		hash := sha256.Sum256(bytes)

		signaturestring := annotations["marketplace.redhat.com/signature"]
		signaturehex, err := hex.DecodeString(signaturestring)
		if err != nil {
			return errors.Wrap(err, "signature is malformed, can not hex decode")
		}

		var rsaPublicKey *rsa.PublicKey
		var ok bool
		rsaPublicKey, ok = pubCert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return errors.Wrap(err, "Unable to parse public key")
		}

		err = rsa.VerifyPSS(rsaPublicKey, crypto.SHA256, hash[:], signaturehex, nil)
		if err != nil {
			return errors.Wrap(err, "failed to VerifyPSS")
		}
	}
	return nil
}
