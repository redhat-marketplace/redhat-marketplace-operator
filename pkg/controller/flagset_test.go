package controller

import (
	"testing"

	"github.com/spf13/pflag"
)

// Test if flags have been set
func TestControllerFlags(t *testing.T) {
	total := pflag.NewFlagSet("all", pflag.ExitOnError)
	controllerFlagSet := ProvideControllerFlagSet()

	total.AddFlagSet((*pflag.FlagSet)(controllerFlagSet))

	if !total.HasFlags() {
		t.Errorf("no flags on flagset")
	}
}
