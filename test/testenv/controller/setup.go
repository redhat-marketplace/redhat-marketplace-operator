package controller

import (
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const Namespace = "openshift-redhat-marketplace"

var K8sClient client.Client
var CC reconcileutils.ClientCommandRunner

var SucceedOrAlreadyExist types.GomegaMatcher = SatisfyAny(
	Succeed(),
	WithTransform(errors.IsAlreadyExists, BeTrue()),
)

var IsNotFound types.GomegaMatcher = WithTransform(errors.IsNotFound, BeTrue())
