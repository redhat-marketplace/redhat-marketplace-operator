package controller

import (
	"github.ibm.com/symposium/redhat-marketplace-operator/pkg/controller/razeedeployment"
)

type RazeeDeployController ControllerDefinition

func ProvideRazeeDeployController() *RazeeDeployController {
	return &RazeeDeployController{
		Add:     razeedeployment.Add,
		FlagSet: razeedeployment.FlagSet,
	}
}
