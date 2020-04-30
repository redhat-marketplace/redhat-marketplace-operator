// Copyright 2020 IBM Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
