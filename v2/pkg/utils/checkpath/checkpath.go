// Copyright 2021 Google LLC
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

package checkpath

import (
	"reflect"
	"strings"

	"emperror.dev/errors"
	"github.com/oliveagle/jsonpath"
)

type CheckUpdatePath struct {
	Root   string
	Paths  []*CheckUpdatePath
	Update func()
}

func (c *CheckUpdatePath) Eval(old, new interface{}) (bool, string, error) {
	if c.Root == "" {
		c.Root = "$"
	}

	oldRoot, err := jsonpath.JsonPathLookup(old, c.Root)
	oldMissing := false

	if err != nil {
		if !strings.Contains(err.Error(), "not found in object") &&
			!strings.Contains(err.Error(), "out of range") {
			return false, c.Root, errors.WrapWithDetails(err, "error occurred", "path", c.Root)
		}
		oldMissing = true
	}

	newRoot, err := jsonpath.JsonPathLookup(new, c.Root)
	newMissing := false

	if err != nil {
		if !strings.Contains(err.Error(), "not found in object") &&
			!strings.Contains(err.Error(), "out of range") {
			return false, c.Root, errors.WrapWithDetails(err, "error occurred", "path", c.Root)
		}

		newMissing = true
	}

	switch {
	case oldMissing && newMissing:
		return false, c.Root, nil
	case oldMissing && !newMissing:
		fallthrough
	case !oldMissing && newMissing:
		if c.Update != nil {
			c.Update()
		}
		return true, c.Root, nil
	default:
	}

	// if the path size is 0, we're just comparing the root; use deep equal
	// and return the result
	if len(c.Paths) == 0 {
		if reflect.DeepEqual(oldRoot, newRoot) {
			return false, c.Root, nil
		}

		// we want to update if there has been a change and c.Update is defined
		if c.Update != nil {
			c.Update()
		}

		return true, c.Root, nil
	}

	hasChange := false
	for _, path := range c.Paths {
		changed, val, err := path.Eval(oldRoot, newRoot)

		if err != nil {
			return false, val, err
		}
		if changed {
			hasChange = true
			if c.Update != nil {
				c.Update()
			}
			break
		}
	}

	return hasChange, c.Root, nil
}
