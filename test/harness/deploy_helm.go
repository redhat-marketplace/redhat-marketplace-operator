package harness

import (
	"io"

	"emperror.dev/errors"
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
	buildCmd := GetCommand("./testbin/skaffold", "build", "--default-repo", e.ImageRegistry, "--namespace", e.Namespace, "-q")
	deployCmd := GetCommand("./testbin/skaffold", "deploy", "--build-artifacts", "-")
	r, w := io.Pipe()

	buildCmd.Stdout = w
	deployCmd.Stdin = r

	buildCmd.Start()
	deployCmd.Start()
	err := buildCmd.Wait()
	w.Close()
	err2 := deployCmd.Wait()
	r.Close()

	if err != nil || err2 != nil {
		return errors.Combine(err, err2)
	}

	return nil
}

func (e *deployHelm) Teardown(h *TestHarness) error {
	cmd := GetCommand("./testbin/skaffold", "delete", "--default-repo", "$IMAGE_REGISTRY", "--namespace", "$NAMESPACE")
	err := cmd.Run()
	out, _ := cmd.CombinedOutput()

	if err != nil {
		return errors.Wrap(err, string(out))
	}

	return nil
}
