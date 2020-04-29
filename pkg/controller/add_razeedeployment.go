package controller

import (
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/controller/razeedeployment"
)

type RazeeDeployController ControllerDefinition

func ProvideRazeeDeployController() *RazeeDeployController {
	return &RazeeDeployController{
		Add:     razeedeployment.Add,
		FlagSet: razeedeployment.FlagSet,
	}
}
