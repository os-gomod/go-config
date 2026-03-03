// Package validate provides configuration validation utilities.
// Uses compiled validation plans to minimize runtime overhead.
package validate

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/os-gomod/go-config/types"
)

// Validator validates configuration values.
type Validator interface {
	// Validate validates a value against rules.
	Validate(ctx context.Context, value any) error

	// Name returns the validator name.
	Name() string
}

// Rule represents a validation rule.
type Rule struct {
	Key       string
	Validator Validator
	Required  bool
	Message   string
}

// Plan is a compiled validation plan.
type Plan struct {
	rules    []Rule
	keyIndex map[string]int // Quick lookup by key
}

// NewPlan creates a new validation plan.
func NewPlan(rules ...Rule) *Plan {
	p := &Plan{
		rules:    rules,
		keyIndex: make(map[string]int, len(rules)),
	}

	for i, rule := range rules {
		p.keyIndex[rule.Key] = i
	}

	return p
}

// Validate validates a configuration map against the plan.
func (p *Plan) Validate(ctx context.Context, config map[string]types.Value) error {
	errs := make([]error, 0)

	for _, rule := range p.rules {
		value, exists := config[rule.Key]

		if !exists {
			if rule.Required {
				errs = append(errs, &ValidationError{
					Key:     rule.Key,
					Rule:    "required",
					Message: rule.Message,
				})
			}

			continue
		}

		// Some rules (for example Required) are presence-only and do not need
		// a value validator.
		if rule.Validator == nil {
			continue
		}

		if err := rule.Validator.Validate(ctx, value.Raw()); err != nil {
			errs = append(errs, &ValidationError{
				Key:     rule.Key,
				Rule:    rule.Validator.Name(),
				Message: rule.Message,
				Cause:   err,
			})
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return &ValidationErrors{Errors: errs}
}

// ValidationError represents a single validation error.
type ValidationError struct {
	Key     string
	Rule    string
	Message string
	Cause   error
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	msg := fmt.Sprintf("validation failed for key '%s': %s", e.Key, e.Rule)
	if e.Message != "" {
		msg += " - " + e.Message
	}
	if e.Cause != nil {
		msg += ": " + e.Cause.Error()
	}

	return msg
}

// Unwrap returns the underlying cause.
func (e *ValidationError) Unwrap() error {
	return e.Cause
}

// ValidationErrors represents multiple validation errors.
type ValidationErrors struct {
	Errors []error
}

// Error implements the error interface.
func (e *ValidationErrors) Error() string {
	if len(e.Errors) == 0 {
		return "validation failed"
	}

	var sb strings.Builder
	sb.WriteString("validation failed:\n")
	for _, err := range e.Errors {
		sb.WriteString("  - ")
		sb.WriteString(err.Error())
		sb.WriteString("\n")
	}

	return sb.String()
}

// Builder builds validation plans.
type Builder struct {
	rules []Rule
}

// NewBuilder creates a new validation plan builder.
func NewBuilder() *Builder {
	return &Builder{
		rules: make([]Rule, 0),
	}
}

// Required adds a required field rule.
func (b *Builder) Required(key, message string) *Builder {
	b.rules = append(b.rules, Rule{
		Key:      key,
		Required: true,
		Message:  message,
	})

	return b
}

// Min adds a minimum value rule.
func (b *Builder) Min(key string, minValue int, message string) *Builder {
	b.rules = append(b.rules, Rule{
		Key:       key,
		Validator: &MinValidator{Min: minValue},
		Message:   message,
	})

	return b
}

// Max adds a maximum value rule.
func (b *Builder) Max(key string, maxValue int, message string) *Builder {
	b.rules = append(b.rules, Rule{
		Key:       key,
		Validator: &MaxValidator{Max: maxValue},
		Message:   message,
	})

	return b
}

// Range adds a range rule.
func (b *Builder) Range(key string, minValue, maxValue int, message string) *Builder {
	b.rules = append(b.rules, Rule{
		Key:       key,
		Validator: &RangeValidator{Min: minValue, Max: maxValue},
		Message:   message,
	})

	return b
}

// Pattern adds a regex pattern rule.
func (b *Builder) Pattern(key, pattern, message string) *Builder {
	re := regexp.MustCompile(pattern)
	b.rules = append(b.rules, Rule{
		Key:       key,
		Validator: &PatternValidator{Pattern: re},
		Message:   message,
	})

	return b
}

// Enum adds an enumeration rule.
func (b *Builder) Enum(key string, values []string, message string) *Builder {
	b.rules = append(b.rules, Rule{
		Key:       key,
		Validator: &EnumValidator{Values: values},
		Message:   message,
	})

	return b
}

// Custom adds a custom validator rule.
func (b *Builder) Custom(key string, validator Validator, message string) *Builder {
	b.rules = append(b.rules, Rule{
		Key:       key,
		Validator: validator,
		Message:   message,
	})

	return b
}

// Build creates the validation plan.
func (b *Builder) Build() *Plan {
	return NewPlan(b.rules...)
}

// Built-in validators

// RequiredValidator checks if a value is present.
type RequiredValidator struct{}

func (v *RequiredValidator) Name() string { return "required" }

func (v *RequiredValidator) Validate(_ context.Context, value any) error {
	if value == nil {
		return errors.New("value is required")
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.String:
		if rv.String() == "" {
			return errors.New("value is required")
		}
	case reflect.Slice, reflect.Map:
		if rv.Len() == 0 {
			return errors.New("value is required")
		}
	}

	return nil
}

// MinValidator checks if a value is at least a minimum.
type MinValidator struct {
	Min int
}

func (v *MinValidator) Name() string { return "min" }

func (v *MinValidator) Validate(_ context.Context, value any) error {
	return validateBoundary(value, v.Min, true)
}

// MaxValidator checks if a value is at most a maximum.
type MaxValidator struct {
	Max int
}

func (v *MaxValidator) Name() string { return "max" }

func (v *MaxValidator) Validate(_ context.Context, value any) error {
	return validateBoundary(value, v.Max, false)
}

func validateBoundary(value any, boundary int, minMode bool) error {
	check := func(actual int64) bool {
		if minMode {
			return actual < int64(boundary)
		}

		return actual > int64(boundary)
	}

	message := "at least"
	if !minMode {
		message = "at most"
	}

	switch val := value.(type) {
	case int:
		if check(int64(val)) {
			return fmt.Errorf("value must be %s %d", message, boundary)
		}
	case int64:
		if check(val) {
			return fmt.Errorf("value must be %s %d", message, boundary)
		}
	case float64:
		if minMode {
			if val < float64(boundary) {
				return fmt.Errorf("value must be %s %d", message, boundary)
			}

			return nil
		}

		if val > float64(boundary) {
			return fmt.Errorf("value must be %s %d", message, boundary)
		}

	case string:
		if check(int64(len(val))) {
			return fmt.Errorf("string length must be %s %d", message, boundary)
		}
	}

	return nil
}

// RangeValidator checks if a value is within a range.
type RangeValidator struct {
	Min int
	Max int
}

func (v *RangeValidator) Name() string { return "range" }

func (v *RangeValidator) Validate(_ context.Context, value any) error {
	switch val := value.(type) {
	case int:
		if val < v.Min || val > v.Max {
			return fmt.Errorf("value must be between %d and %d", v.Min, v.Max)
		}
	case int64:
		if val < int64(v.Min) || val > int64(v.Max) {
			return fmt.Errorf("value must be between %d and %d", v.Min, v.Max)
		}
	case float64:
		if val < float64(v.Min) || val > float64(v.Max) {
			return fmt.Errorf("value must be between %d and %d", v.Min, v.Max)
		}
	}

	return nil
}

// PatternValidator checks if a string matches a regex pattern.
type PatternValidator struct {
	Pattern *regexp.Regexp
}

func (v *PatternValidator) Name() string { return "pattern" }

func (v *PatternValidator) Validate(_ context.Context, value any) error {
	s, ok := value.(string)
	if !ok {
		return errors.New("value must be a string")
	}

	if !v.Pattern.MatchString(s) {
		return fmt.Errorf("value must match pattern %s", v.Pattern.String())
	}

	return nil
}

// EnumValidator checks if a value is in a list of allowed values.
type EnumValidator struct {
	Values []string
}

func (v *EnumValidator) Name() string { return "enum" }

func (v *EnumValidator) Validate(_ context.Context, value any) error {
	s, ok := value.(string)
	if !ok {
		return errors.New("value must be a string")
	}

	for _, allowed := range v.Values {
		if s == allowed {
			return nil
		}
	}

	return fmt.Errorf("value must be one of: %s", strings.Join(v.Values, ", "))
}

// EmailValidator checks if a value is a valid email.
type EmailValidator struct{}

func (v *EmailValidator) Name() string { return "email" }

func (v *EmailValidator) Validate(_ context.Context, value any) error {
	s, ok := value.(string)
	if !ok {
		return errors.New("value must be a string")
	}

	// Simple email validation
	if !strings.Contains(s, "@") || !strings.Contains(s, ".") {
		return errors.New("invalid email format")
	}

	atIndex := strings.Index(s, "@")
	dotIndex := strings.LastIndex(s, ".")

	if atIndex < 1 || dotIndex < atIndex+2 || dotIndex >= len(s)-1 {
		return errors.New("invalid email format")
	}

	return nil
}

// URLValidator checks if a value is a valid URL.
type URLValidator struct{}

func (v *URLValidator) Name() string { return "url" }

func (v *URLValidator) Validate(_ context.Context, value any) error {
	s, ok := value.(string)
	if !ok {
		return errors.New("value must be a string")
	}

	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return errors.New("URL must start with http:// or https://")
	}

	return nil
}

// DurationValidator checks if a value is a valid duration.
type DurationValidator struct {
	Min time.Duration
	Max time.Duration
}

func (v *DurationValidator) Name() string { return "duration" }

func (v *DurationValidator) Validate(_ context.Context, value any) error {
	d, ok := value.(time.Duration)
	if !ok {
		return errors.New("value must be a duration")
	}

	if v.Min > 0 && d < v.Min {
		return fmt.Errorf("duration must be at least %s", v.Min)
	}

	if v.Max > 0 && d > v.Max {
		return fmt.Errorf("duration must be at most %s", v.Max)
	}

	return nil
}

// StructValidator validates structs using tags.
type StructValidator struct {
	cache sync.Map // Cache validation metadata
}

// NewStructValidator creates a new struct validator.
func NewStructValidator() *StructValidator {
	return &StructValidator{}
}

func (v *StructValidator) Name() string { return "struct" }

func (v *StructValidator) Validate(ctx context.Context, value any) error {
	return v.validateStruct(ctx, reflect.ValueOf(value))
}

func (v *StructValidator) validateStruct(ctx context.Context, rv reflect.Value) error {
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Struct {
		return errors.New("value must be a struct")
	}

	rt := rv.Type()

	// Check cache for validation metadata
	var meta *structMeta
	if cached, ok := v.cache.Load(rt); ok {
		meta = cached.(*structMeta)
	} else {
		meta = v.buildMeta(rt)
		v.cache.Store(rt, meta)
	}

	var errs []error

	for _, field := range meta.fields {
		fv := rv.FieldByIndex(field.index)

		for _, rule := range field.rules {
			if err := rule.Validate(ctx, fv.Interface()); err != nil {
				errs = append(errs, &ValidationError{
					Key:     field.name,
					Rule:    rule.Name(),
					Message: err.Error(),
				})
			}
		}
	}

	if len(errs) > 0 {
		return &ValidationErrors{Errors: errs}
	}

	return nil
}

// structMeta holds cached validation metadata for a struct type.
type structMeta struct {
	fields []fieldMeta
}

// fieldMeta holds validation metadata for a struct field.
type fieldMeta struct {
	name  string
	index []int
	rules []Validator
}

// buildMeta builds validation metadata for a struct type.
func (v *StructValidator) buildMeta(rt reflect.Type) *structMeta {
	meta := &structMeta{
		fields: make([]fieldMeta, 0),
	}

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)

		// Skip unexported fields
		if field.PkgPath != "" {
			continue
		}

		tag := field.Tag.Get("validate")
		if tag == "" {
			continue
		}

		name := field.Tag.Get("config")
		if name == "" {
			name = strings.ToLower(field.Name)
		}

		rules := parseValidateTag(tag)

		meta.fields = append(meta.fields, fieldMeta{
			name:  name,
			index: field.Index,
			rules: rules,
		})
	}

	return meta
}

// parseValidateTag parses validation tags into validators.
func parseValidateTag(tag string) []Validator {
	rules := make([]Validator, 0)

	parts := strings.Split(tag, ",")
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		key := strings.TrimSpace(kv[0])
		var value string
		if len(kv) > 1 {
			value = strings.TrimSpace(kv[1])
		}

		switch key {
		case "required":
			rules = append(rules, &RequiredValidator{})
		case "min":
			if n, err := strconv.Atoi(value); err == nil {
				rules = append(rules, &MinValidator{Min: n})
			}
		case "max":
			if n, err := strconv.Atoi(value); err == nil {
				rules = append(rules, &MaxValidator{Max: n})
			}
		case "email":
			rules = append(rules, &EmailValidator{})
		case "url":
			rules = append(rules, &URLValidator{})
		case "enum":
			values := strings.Split(value, "|")
			rules = append(rules, &EnumValidator{Values: values})
		case "pattern":
			if re, err := regexp.Compile(value); err == nil {
				rules = append(rules, &PatternValidator{Pattern: re})
			}
		}
	}

	return rules
}

// ValidatorFunc is an adapter for using functions as validators.
type ValidatorFunc func(ctx context.Context, value any) error

func (f ValidatorFunc) Name() string { return "custom" }

func (f ValidatorFunc) Validate(ctx context.Context, value any) error {
	return f(ctx, value)
}

// AlphanumericValidator checks if a string contains only alphanumeric characters.
type AlphanumericValidator struct{}

func (v *AlphanumericValidator) Name() string { return "alphanumeric" }

func (v *AlphanumericValidator) Validate(_ context.Context, value any) error {
	s, ok := value.(string)
	if !ok {
		return errors.New("value must be a string")
	}

	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return errors.New("value must contain only alphanumeric characters")
		}
	}

	return nil
}

// UUIDValidator checks if a string is a valid UUID.
type UUIDValidator struct{}

func (v *UUIDValidator) Name() string { return "uuid" }

func (v *UUIDValidator) Validate(_ context.Context, value any) error {
	s, ok := value.(string)
	if !ok {
		return errors.New("value must be a string")
	}

	// UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	if len(s) != 36 {
		return errors.New("invalid UUID length")
	}

	for i, r := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if r != '-' {
				return errors.New("invalid UUID format")
			}
		} else {
			if !unicode.IsDigit(r) && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
				return errors.New("invalid UUID format")
			}
		}
	}

	return nil
}
