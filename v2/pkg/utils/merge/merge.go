package merge

import (
	"reflect"
)

// MergeSliceByFieldName is a mergo transformer that will
// merge slices of structs based on a field in the struct
type MergeSliceByFieldName struct {
	FieldName string
}

func (s MergeSliceByFieldName) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ.Kind() != reflect.Slice {
		return nil
	}

	fieldName := s.FieldName
	fieldType := typ.Elem()

	if fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
	}

	if fieldType.Kind() != reflect.Struct {
		return nil
	}

	if _, present := fieldType.FieldByName(fieldName); !present {
		return nil
	}

	return func(dst, src reflect.Value) error {
		results := map[string]reflect.Value{}

		if !dst.CanSet() {
			return nil
		}

		for i := 0; i < dst.Len(); i++ {
			val := dst.Index(i)

			if val.Kind() == reflect.Ptr {
				val = val.Elem()
				target := val.FieldByName(fieldName).String()
				results[target] = val.Addr()
				continue
			}

			target := val.FieldByName(fieldName).String()
			results[target] = val
		}

		for i := 0; i < src.Len(); i++ {
			val := src.Index(i)

			if val.Kind() == reflect.Ptr {
				val = val.Elem()
				target := val.FieldByName(fieldName).String()
				results[target] = val.Addr()
				continue
			}

			target := val.FieldByName(fieldName).String()
			results[target] = val
		}

		vals := reflect.New(dst.Type())
		valsPtr := vals.Elem()
		for _, v := range results {
			valsPtr = reflect.Append(valsPtr, v)
		}

		dst.Set(valsPtr)
		return nil
	}
}

type MergeSliceFunc struct {
	SliceType interface{}
	FuncName  string
}

func (s MergeSliceFunc) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	var method reflect.Method
	var ok bool

	if typ != reflect.TypeOf(s.SliceType) {
		return nil
	}

	convertToPtr := false
	convertToStruct := false
	method, ok = typ.MethodByName(s.FuncName)
	if !ok {
		if typ.Kind() != reflect.Ptr {
			method, ok = reflect.PtrTo(typ).MethodByName(s.FuncName)
			convertToPtr = true
		}

		if typ.Kind() == reflect.Ptr {
			method, ok = typ.Elem().MethodByName(s.FuncName)
			convertToStruct = true
		}

		if !ok {
			return nil
		}
	}

	funcType := method.Func.Type()

	if funcType.NumIn() != 2 {
		return nil
	}

	return func(dst, src reflect.Value) error {
		if !dst.CanSet() {
			return nil
		}

		for i := 0; i < src.Len(); i++ {
			val := src.Index(i)

			if convertToPtr {
				dst = dst.Addr()
			}
			if convertToStruct {
				dst = dst.Elem()
			}

			method.Func.Call([]reflect.Value{dst, val})
		}

		return nil
	}
}
