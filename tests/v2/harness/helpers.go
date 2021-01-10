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

package harness

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func GetCommand(cmd string, args ...string) *exec.Cmd {
	rootDir, _ := GetRootDirectory()
	command := exec.Command(cmd, args...)
	command.Dir = rootDir
	command.Env = os.Environ()
	return command
}

func GetRootDirectory() (string, error) {
	cwd, err := os.Getwd()

	if err != nil {
		return "", err
	}

	paths := strings.Split(cwd, string(filepath.Separator))
	count := 0
	for i := len(paths) - 1; i >= 0; i-- {
		if paths[i] == "redhat-marketplace-operator" {
			break
		}
		count = count + 1
	}

	result := cwd

	for i := 0; i < count; i++ {
		result = filepath.Join(result, "..")
	}

	return result, nil
}
