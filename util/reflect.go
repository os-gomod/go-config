package util

import "reflect"

// Indirect unwraps pointers.
func Indirect(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	return v
}

// FindField finds a struct field by name or tag.
func FindField(v reflect.Value, name string) (reflect.Value, bool) {
	v = Indirect(v)
	if v.Kind() != reflect.Struct {
		return reflect.Value{}, false
	}

	return findFieldInStruct(v, name)
}

func findFieldInStruct(v reflect.Value, name string) (reflect.Value, bool) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)

		// Match by exact field name
		if f.Name == name {
			return v.Field(i), true
		}
		// Match by config tag
		if f.Tag.Get("config") == name {
			return v.Field(i), true
		}
	}
	return reflect.Value{}, false
}
