package controller_test

import (
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/api/errors"
)

var SucceedOrAlreadyExist types.GomegaMatcher = SatisfyAny(
	Succeed(),
	WithTransform(errors.IsAlreadyExists, BeTrue()),
)

var IsNotFound types.GomegaMatcher = WithTransform(errors.IsNotFound, BeTrue())
