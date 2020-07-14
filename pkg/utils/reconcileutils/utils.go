package reconcileutils

import "reflect"

func isNil(i interface{}) bool {
	return i == nil || reflect.ValueOf(i).IsNil()
}

func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}

	return false
}
