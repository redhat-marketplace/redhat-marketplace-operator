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
			t.Error("%s is not equal to %s", testVar, "1")
		}
	})

	t.Run("env var does not exist", func(t *testing.T) {
		testVar := Getenv("DOESNOEXIST", "2")
		if testVar != "2" {
			t.Error("%s is not equal to %s", testVar, "2")
		}
	})
}
