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

	buildCmd := GetCommand("./testbin/skaffold", "build", "--default-repo", e.ImageRegistry, "-q")
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
