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

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/client"
	"github.com/spf13/cobra"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	rootCmd = &cobra.Command{
		Use:   "authvalid",
		Short: "Checks if kube auth is still valid.",
		Run:   run,
	}

	group, kind, version, namespace string
	retry                           int64

	log = logf.Log.WithName("authvalid_cmd")
)

func init() {
	logf.SetLogger(zap.Logger())
	rootCmd.Flags().StringVar(&group, "group", "", "kube group to list")
	rootCmd.Flags().StringVar(&kind, "kind", "Pod", "kube kind to list")
	rootCmd.Flags().StringVar(&version, "version", "v1", "kube version to list")
	rootCmd.Flags().StringVar(&namespace, "namespace", "", "namespace to list")
	rootCmd.Flags().Int64Var(&retry, "retry", 30, "retry count")
}

func run(cmd *cobra.Command, args []string) {
	log.Info("starting up with vars",
		"group", group,
		"kind", kind,
		"namespace", namespace,
		"retry", retry,
	)

	authChecker, err := InitializeAuthChecker(client.AuthCheckerConfig{
		Namespace: namespace,
		RetryTime: time.Duration(retry) * time.Second,
		Kind:      kind,
		Group:     group,
		Version:   version,
	})
	if err != nil {
		log.Error(err, "error starting auth checker")
		er(err)
	}

	err = authChecker.Run(cmd.Context())

	if err != nil {
		log.Error(err, "error checking auth")
		er(err)
	}
}

func er(msg interface{}) {
	fmt.Println("Error:", msg)
	os.Exit(1)
}

func main() {
	err := rootCmd.Execute()

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	os.Exit(0)
}
