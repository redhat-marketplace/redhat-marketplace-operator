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

package sign

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"emperror.dev/errors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/signer"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	sigsyaml "sigs.k8s.io/yaml"
)

var log = logf.Log.WithName("signer_sign_cmd")

var f, publickey, privatekey, privatekeypassword string

var SignCmd = &cobra.Command{
	Use:   "sign",
	Short: "Sign the yaml",
	Long:  `Sign the yaml. Takes yaml file, public key and private key as args`,
	Run: func(cmd *cobra.Command, args []string) {

		if publickey == "" {
			log.Error(errors.New("public key not provided"), "public key not provided")
			os.Exit(1)
		}

		if privatekey == "" {
			log.Error(errors.New("private key not provided"), "private key not provided")
			os.Exit(1)
		}

		var file io.ReadCloser
		var err error
		if signer.IsInputFromPipe() {
			file = os.Stdin
		} else {
			file, err = signer.OpenInputFile(f)
			if err != nil {
				log.Error(err, "Could not open input file")
				os.Exit(1)
			}
		}

		uobjs, err := signer.Decode(file)
		file.Close()
		if err != nil {
			log.Error(err, "Could not Decode input yaml contents")
			os.Exit(1)
		}

		pubKey, err := ioutil.ReadFile(publickey)
		if err != nil {
			log.Error(err, "Could not read public key file")
			os.Exit(1)
		}

		password := ""
		if privatekeypassword != "" {
			password = privatekeypassword
		}

		privKey, err := signer.PrivateKeyFromPemFile(privatekey, password)
		if err != nil {
			log.Error(err, "Could not get private key from pem file")
			os.Exit(1)
		}

		for i, uobj := range uobjs {
			// Reduce Object to GVK+Spec and sign that content
			bytes, err := signer.UnstructuredToGVKSpecBytes(uobj)
			if err != nil {
				log.Error(err, "could not MarshalJSON")
				os.Exit(1)
			}

			hash := sha256.Sum256(bytes)

			signature, err := rsa.SignPSS(rand.Reader, privKey, crypto.SHA256, hash[:], nil)
			if err != nil {
				log.Error(err, "could not sign")
				os.Exit(1)
			}

			annotations := make(map[string]string)

			annotations["marketplace.redhat.com/signature"] = fmt.Sprintf("%x", signature)
			annotations["marketplace.redhat.com/publickey"] = fmt.Sprintf("%s", pubKey)

			uobjs[i].SetAnnotations(annotations)
		}

		uList := unstructured.UnstructuredList{}
		uList.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "List",
		})
		uList.Items = uobjs

		uListBytes, err := uList.MarshalJSON()
		if err != nil {
			log.Error(err, "could not MarshalJSON")
			os.Exit(1)
		}

		yamlout, err := sigsyaml.JSONToYAML(uListBytes)
		if err != nil {
			log.Error(err, "could not JSONToYAML")
			os.Exit(1)
		}

		fmt.Printf("%s", yamlout)

		os.Exit(0)
	},
}

func init() {
	SignCmd.Flags().StringVar(&f, "f", "", "input yaml file")
	SignCmd.Flags().StringVar(&publickey, "publickey", "", "public key")
	SignCmd.Flags().StringVar(&privatekey, "privatekey", "", "private key")
	SignCmd.Flags().StringVar(&privatekeypassword, "privatekeypassword", "", "private key password")
}
