package controller

import (
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/test/testenv/harness"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const Namespace = "openshift-redhat-marketplace"

var TestHarness *harness.TestHarness
var K8sClient client.Client
var CC reconcileutils.ClientCommandRunner

var SucceedOrAlreadyExist types.GomegaMatcher = SatisfyAny(
	Succeed(),
	WithTransform(errors.IsAlreadyExists, BeTrue()),
)

var IsNotFound types.GomegaMatcher = WithTransform(errors.IsNotFound, BeTrue())
