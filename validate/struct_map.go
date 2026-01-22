package validate

import "reflect"

// BuildStructKeyMap maps config keys to struct field paths.
func BuildStructKeyMap(prefix string, s any, out map[string]string) {
	rv := reflect.ValueOf(s)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		processFieldMapping(prefix, field, rv.Field(i), out)
	}
}

func processFieldMapping(prefix string, field reflect.StructField, fieldValue reflect.Value, out map[string]string) {
	cfg := field.Tag.Get("config")
	if cfg == "" {
		return
	}

	key := buildFullKey(prefix, cfg)
	fieldPath := buildFullKey(prefix, field.Name)
	out[key] = fieldPath

	if field.Type.Kind() == reflect.Struct && field.Type.String() != "time.Duration" {
		BuildStructKeyMap(key, fieldValue.Interface(), out)
	}
}
