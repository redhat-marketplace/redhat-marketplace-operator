package controller

import (
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/controller/remoteresources3"
	"github.com/spf13/pflag"
)

type RemoteResourceS3Controller ControllerDefinition

func ProvideRemoteResourceS3Controller() *RemoteResourceS3Controller {
	return &RemoteResourceS3Controller{
		Add:     remoteresources3.Add,
		FlagSet: func() *pflag.FlagSet { return nil },
	}
}
