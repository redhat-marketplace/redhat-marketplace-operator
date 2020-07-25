package controller

import (
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/controller/remoteresources3"
	"github.com/spf13/pflag"
)

type RemoteResourceS3Controller struct {
	*baseDefinition
}

func ProvideRemoteResourceS3Controller() *RemoteResourceS3Controller {
	return &RemoteResourceS3Controller{
		baseDefinition: &baseDefinition{
			AddFunc:     remoteresources3.Add,
			FlagSetFunc: func() *pflag.FlagSet { return nil },
		},
	}
}
