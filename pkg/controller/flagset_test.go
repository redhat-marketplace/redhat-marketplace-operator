package controller

import (
	"testing"

	"github.com/spf13/pflag"
)

func TestControllerFlags(t *testing.T) {
	flagsets := FlagSets()

	total := pflag.NewFlagSet("all", pflag.ExitOnError)

	for _, flags := range flagsets {
		total.AddFlagSet(flags)
	}

	if !total.HasFlags() {
		t.Errorf("no flags on flagset")
	}
}
