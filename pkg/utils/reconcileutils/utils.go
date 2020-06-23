package reconcileutils

import "reflect"

func isNil(i interface{}) bool {
	return i == nil || reflect.ValueOf(i).IsNil()
}
