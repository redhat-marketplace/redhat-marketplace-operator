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

package utils

import (
	"os"
	"testing"
)

func TestEnvUtils(t *testing.T) {
	os.Setenv("TEST", "1")

	t.Run("env var exists", func(t *testing.T) {
		testVar := Getenv("TEST", "2")
		if testVar != "1" {
			t.Errorf("%s is not equal to %s", testVar, "1")
		}
	})

	t.Run("env var does not exist", func(t *testing.T) {
		testVar := Getenv("DOESNOEXIST", "2")
		if testVar != "2" {
			t.Errorf("%s is not equal to %s", testVar, "2")
		}
	})
}
