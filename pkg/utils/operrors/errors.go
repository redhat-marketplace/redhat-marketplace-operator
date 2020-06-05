package operrors

import (
	emperrors "emperror.dev/errors"
)

const DefaultStorageClassNotFound = emperrors.Sentinel("default storage class not found")
const MultipleDefaultStorageClassFound = emperrors.Sentinel("multiple default storage found")
