package reconcileutils

import "emperror.dev/errors"

// ErrNilObject is returned if a nil object is provided when
// a non-nil object was expected
const ErrNilObject = errors.Sentinel("object provided is nil")
