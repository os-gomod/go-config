package bind

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/os-gomod/go-config/util"
	"github.com/os-gomod/go-config/validate"
)

// Bind maps flat config keys to struct fields.
func Bind(data map[string]any, dst any) error {
	rv, err := validateDestination(dst)
	if err != nil {
		return err
	}

	return bindFields(rv, data)
}

func validateDestination(dst any) (reflect.Value, error) {
	rv := reflect.ValueOf(dst)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return reflect.Value{}, fmt.Errorf("destination must be non-nil pointer")
	}
	return rv.Elem(), nil
}

func bindFields(rv reflect.Value, data map[string]any) error {
	for k, v := range data {
		if err := bindField(rv, k, v); err != nil {
			return err
		}
	}
	return nil
}

func bindField(rv reflect.Value, key string, value any) error {
	// Split the key by dots to handle nested structures
	parts := strings.Split(key, ".")

	// Navigate through the nested structure
	current := rv
	for i, part := range parts {
		// Find the field matching this part (by name or config tag)
		field, ok := util.FindField(current, part)
		if !ok {
			// if opt.Strict {
			// 	return fmt.Errorf("unknown config key: %s", key)
			// }
			return nil
		}

		// If this is the last part, set the value
		if i == len(parts)-1 {
			if err := util.Convert(field, value); err != nil {
				return fmt.Errorf("bind %s: %w", key, err)
			}
			return nil
		}

		// Otherwise, navigate into the field if it's a struct
		field = util.Indirect(field)
		if field.Kind() != reflect.Struct {
			// Can't navigate further, skip
			return nil
		}
		current = field
	}

	return nil
}

// BindAndValidate binds config and runs validation.
func BindAndValidate(
	data map[string]any,
	dst any,
	validator *validate.Manager,
) error {
	if err := Bind(data, dst); err != nil {
		return err
	}

	return runValidation(dst, data, validator)
}

func runValidation(dst any, data map[string]any, validator *validate.Manager) error {
	// Struct-level validation
	if err := validator.ValidateStruct(dst); err != nil {
		return err
	}

	// Field-level validation
	if err := validator.Validate(data); err != nil {
		return err
	}

	return nil
}
