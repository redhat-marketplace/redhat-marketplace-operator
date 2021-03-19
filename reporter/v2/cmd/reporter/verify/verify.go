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

package verify

import (
	"fmt"
	"io"
	"os"

	"emperror.dev/errors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils/signer"
	"github.com/spf13/cobra"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("signer_verify_cmd")

var f, ca string

var VerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify the yaml",
	Long:  `Verify the yaml. Takes yaml file, ca as args`,
	Run: func(cmd *cobra.Command, args []string) {

		if ca == "" {
			log.Error(errors.New("ca not provided"), "ca not provided")
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

		caCert, err := signer.CertificateFromPemFile(ca)
		if err != nil {
			log.Error(err, "Could not retrieve ca certificate")
			os.Exit(1)
		}

		err = signer.VerifySignatureArray(uobjs, caCert)
		if err != nil {
			fmt.Printf("yaml failed verification")
			log.Error(err, "yaml failed verification")
			os.Exit(1)
		}

		os.Exit(0)
	},
}

func init() {
	VerifyCmd.Flags().StringVar(&f, "f", "", "input yaml file")
	VerifyCmd.Flags().StringVar(&ca, "ca", "", "certificate authority file")
}
