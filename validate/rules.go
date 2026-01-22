package validate

import (
	"fmt"
	"strings"
)

// Rule validates configuration values.
type Rule interface {
	Validate(string, map[string]any) error
}

// StructRule validates the full struct.
type StructRule interface {
	ValidateStruct(any) error
}

// RequiredRule ensures key exists and has a non-zero value.
type RequiredRule struct{}

func (RequiredRule) Validate(key string, data map[string]any) error {
	v, ok := data[key]
	if !ok || v == nil {
		return fmt.Errorf("is required")
	}

	if isEmpty(v) {
		return fmt.Errorf("is required")
	}
	return nil
}

func isEmpty(v any) bool {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t) == ""
	case int, int64:
		return t == 0
	default:
		return false
	}
}

// OneOfRule ensures value is one of allowed.
type OneOfRule struct {
	Allowed []any
}

func (r OneOfRule) Validate(key string, data map[string]any) error {
	v, ok := data[key]
	if !ok {
		return nil
	}

	for _, a := range r.Allowed {
		if v == a {
			return nil
		}
	}
	return fmt.Errorf("value %v not in %v", v, r.Allowed)
}

// CustomRule allows arbitrary validation logic.
type CustomRule func(string, map[string]any) error

func (r CustomRule) Validate(key string, data map[string]any) error {
	return r(key, data)
}
