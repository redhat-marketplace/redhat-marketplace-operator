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
