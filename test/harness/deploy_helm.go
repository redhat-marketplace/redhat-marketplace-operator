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

package harness

import (
	"bytes"
	"fmt"
	"io"

	"github.com/caarlos0/env"
	"k8s.io/apimachinery/pkg/runtime"
)

const FeatureDeployHelm string = "DeployHelm"

type deployHelm struct {
	Namespace     string `env:"NAMESPACE" envDefault:"openshift-redhat-marketplace"`
	ImageRegistry string `env:"IMAGE_REGISTRY"`
	ImageTag      string `env:"OPERATOR_IMAGE_TAG"`

	cleanup []runtime.Object
}

func (e *deployHelm) Name() string {
	return FeatureDeployHelm
}

func (e *deployHelm) Parse() error {
	return env.Parse(e)
}

func (e *deployHelm) HasCleanup() []runtime.Object {
	return e.cleanup
}

func (e *deployHelm) Setup(h *TestHarness) error {
	var buildOut, buildErr, deployOut, deployErr bytes.Buffer

	additionalArgs := []string{"build", "--detect-minikube=true", "--default-repo", e.ImageRegistry, "-q"}
	if e.ImageTag != "" {
		additionalArgs = append(additionalArgs, "-t", e.ImageTag, "--dry-run=true")
	}

	buildCmd := GetCommand("./testbin/skaffold", additionalArgs...)
	deployCmd := GetCommand("./testbin/skaffold", "deploy", fmt.Sprintf("--namespace=%s", e.Namespace), "--build-artifacts", "-")
	r, w := io.Pipe()

	buildCmd.Stdout = w
	buildCmd.Stderr = &buildErr
	deployCmd.Stdin = io.TeeReader(r, &buildOut)
	deployCmd.Stdout = &deployOut
	deployCmd.Stderr = &deployErr

	buildCmd.Start()
	deployCmd.Start()
	err := buildCmd.Wait()
	w.Close()
	err2 := deployCmd.Wait()
	r.Close()

	if err != nil {
		h.logger.Error(err, "failed to build", "output", buildOut.String(), "error", buildErr.String())
		return err
	}

	if err2 != nil {
		h.logger.Error(err2, "failed to deploy ", "output", deployOut.String(), "error", deployErr.String())
		return err2
	}

	return nil
}

func (e *deployHelm) Teardown(h *TestHarness) error {
	cmd := GetCommand("./testbin/skaffold", "delete", "--namespace", e.Namespace)
	cmd.Run()

	return nil
}
