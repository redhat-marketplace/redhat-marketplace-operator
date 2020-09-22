package helpers

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var TestEnv TestEnvContext

type TestEnvContext struct {
	K8SClient client.Client
}
