package validate

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// StructValidator bridges struct tags to Manager rules.
type StructValidator struct {
	mgr *Manager
}

func NewStructValidator(mgr *Manager) *StructValidator {
	return &StructValidator{mgr: mgr}
}

// RegisterStruct parses validation tags and registers rules.
func (v *StructValidator) RegisterStruct(prefix string, s any) error {
	rv := reflect.ValueOf(s)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("expected struct")
	}

	return v.processStructFields(prefix, rv)
}

func (v *StructValidator) processStructFields(prefix string, rv reflect.Value) error {
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if err := v.processField(prefix, field, rv.Field(i)); err != nil {
			return err
		}
	}
	return nil
}

func (v *StructValidator) processField(prefix string, field reflect.StructField, fieldValue reflect.Value) error {
	cfgKey := field.Tag.Get("config")
	if cfgKey == "" {
		return nil
	}

	fullKey := buildFullKey(prefix, cfgKey)

	// Handle nested structs
	if isNestedStruct(field) {
		return v.RegisterStruct(fullKey, fieldValue.Interface())
	}

	// Process validation tags
	tag := field.Tag.Get("validate")
	if tag != "" {
		return v.processValidationTags(fullKey, prefix, tag)
	}

	return nil
}

func isNestedStruct(field reflect.StructField) bool {
	return field.Type.Kind() == reflect.Struct &&
		!field.Anonymous &&
		field.Type.String() != "time.Duration"
}

func buildFullKey(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func (v *StructValidator) processValidationTags(fullKey, prefix, tag string) error {
	rules := strings.Split(tag, ",")
	for _, r := range rules {
		v.registerRuleFromTag(fullKey, prefix, strings.TrimSpace(r))
	}
	return nil
}

func (v *StructValidator) registerRuleFromTag(fullKey, prefix, rule string) {
	switch {
	case rule == "required":
		v.mgr.Register(fullKey, RequiredRule{})

	case strings.HasPrefix(rule, "oneof="):
		v.registerOneOfRule(fullKey, rule)

	case strings.HasPrefix(rule, "min="):
		v.registerMinRule(fullKey, rule)

	case strings.HasPrefix(rule, "max="):
		v.registerMaxRule(fullKey, rule)

	case strings.HasPrefix(rule, "required_if="):
		v.registerRequiredIfRule(fullKey, prefix, rule)
	}
}

func (v *StructValidator) registerOneOfRule(fullKey, rule string) {
	vals := strings.Split(strings.TrimPrefix(rule, "oneof="), " ")
	allowed := make([]any, len(vals))
	for i, val := range vals {
		allowed[i] = val
	}
	v.mgr.Register(fullKey, OneOfRule{Allowed: allowed})
}

func (v *StructValidator) registerMinRule(fullKey, rule string) {
	minVal, _ := strconv.ParseFloat(strings.TrimPrefix(rule, "min="), 64)
	v.mgr.Register(fullKey, CustomRule(func(_ string, data map[string]any) error {
		if val, ok := data[fullKey].(float64); ok && val < minVal {
			return fmt.Errorf("must be >= %v", minVal)
		}
		return nil
	}))
}

func (v *StructValidator) registerMaxRule(fullKey, rule string) {
	maxVal, _ := strconv.ParseFloat(strings.TrimPrefix(rule, "max="), 64)
	v.mgr.Register(fullKey, CustomRule(func(_ string, data map[string]any) error {
		if val, ok := data[fullKey].(float64); ok && val > maxVal {
			return fmt.Errorf("must be <= %v", maxVal)
		}
		return nil
	}))
}

func (v *StructValidator) registerRequiredIfRule(fullKey, prefix, rule string) {
	parts := strings.Split(strings.TrimPrefix(rule, "required_if="), " ")
	if len(parts) != 2 {
		return
	}

	otherKey := buildFullKey(prefix, parts[0])
	expected := parts[1]

	v.mgr.Register(fullKey, CustomRule(func(_ string, data map[string]any) error {
		if fmt.Sprint(data[otherKey]) == expected {
			if _, ok := data[fullKey]; !ok {
				return fmt.Errorf("required when %s=%s", otherKey, expected)
			}
		}
		return nil
	}))
}
