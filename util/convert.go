package util

import (
	"fmt"
	"reflect"
	"strconv"
	"time"
)

// Convert assigns raw into dst if compatible.
func Convert(dst reflect.Value, raw any) error {
	dst = Indirect(dst)

	if !dst.CanSet() {
		return fmt.Errorf("destination not settable")
	}

	if dst.Type() == reflect.TypeOf(time.Duration(0)) {
		return convertToDuration(dst, raw)
	}

	switch dst.Kind() {
	case reflect.String:
		return convertToString(dst, raw)
	case reflect.Int, reflect.Int64:
		return convertToInt(dst, raw)
	case reflect.Bool:
		return convertToBool(dst, raw)
	case reflect.Float64:
		return convertToFloat(dst, raw)
	case reflect.Struct:
		return convertToStruct(dst, raw)
	}

	return fmt.Errorf("cannot convert %T to %s", raw, dst.Type())
}
func convertToDuration(dst reflect.Value, raw any) error {
	switch v := raw.(type) {
	case time.Duration:
		dst.SetInt(int64(v))
		return nil
	case int64:
		dst.SetInt(v)
		return nil
	case int:
		dst.SetInt(int64(v))
		return nil
	case string:
		d, err := time.ParseDuration(v)
		if err != nil {
			return err
		}
		dst.SetInt(int64(d))
		return nil
	}
	return fmt.Errorf("cannot convert %T to time.Duration", raw)
}

func convertToString(dst reflect.Value, raw any) error {
	dst.SetString(fmt.Sprint(raw))
	return nil
}

func convertToInt(dst reflect.Value, raw any) error {
	switch v := raw.(type) {
	case int:
		dst.SetInt(int64(v))
		return nil
	case int64:
		dst.SetInt(v)
		return nil
	case string:
		i, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return err
		}
		dst.SetInt(i)
		return nil
	}
	return fmt.Errorf("cannot convert %T to int", raw)
}

func convertToBool(dst reflect.Value, raw any) error {
	if b, ok := raw.(bool); ok {
		dst.SetBool(b)
		return nil
	}
	return fmt.Errorf("cannot convert %T to bool", raw)
}

func convertToFloat(dst reflect.Value, raw any) error {
	if f, ok := raw.(float64); ok {
		dst.SetFloat(f)
		return nil
	}
	return fmt.Errorf("cannot convert %T to float64", raw)
}

func convertToStruct(dst reflect.Value, raw any) error {
	if dst.Type() == reflect.TypeOf(time.Duration(0)) {
		if s, ok := raw.(string); ok {
			d, err := time.ParseDuration(s)
			if err != nil {
				return err
			}
			dst.Set(reflect.ValueOf(d))
			return nil
		}
	}
	return fmt.Errorf("cannot convert %T to %s", raw, dst.Type())
}
