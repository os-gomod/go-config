package export

import (
	"encoding/json"
	"reflect"
)

func GenerateSchema(s any) (map[string]any, error) {
	rv := unwrapType(reflect.TypeOf(s))
	props := buildProperties(rv)

	return map[string]any{
		"$schema":    "http://json-schema.org/draft-07/schema#",
		"type":       "object",
		"properties": props,
	}, nil
}

func unwrapType(t reflect.Type) reflect.Type {
	if t.Kind() == reflect.Ptr {
		return t.Elem()
	}
	return t
}

func buildProperties(t reflect.Type) map[string]any {
	props := map[string]any{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if key := f.Tag.Get("config"); key != "" {
			props[key] = fieldSchema(f.Type)
		}
	}
	return props
}

func fieldSchema(t reflect.Type) map[string]any {
	switch t.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Int, reflect.Int64:
		return map[string]any{"type": "integer"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.Struct:
		return structSchema(t)
	}
	return map[string]any{}
}

func structSchema(t reflect.Type) map[string]any {
	props := buildProperties(t)
	return map[string]any{"type": "object", "properties": props}
}

func SchemaJSON(s any) ([]byte, error) {
	schema, err := GenerateSchema(s)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(schema, "", "  ")
}
