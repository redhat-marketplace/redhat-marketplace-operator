package controller

import (
	"github.com/spf13/pflag"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/controller/subscription"
)

type OlmSubscriptionController ControllerDefinition

func ProvideOlmSubscriptionController() *OlmSubscriptionController {
	return &OlmSubscriptionController{
		Add:     subscription.Add,
		FlagSet: func() *pflag.FlagSet { return nil },
	}
}
