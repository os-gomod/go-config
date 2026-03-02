// Package reflectutil provides reflection utilities optimized for the configuration system.
package reflectutil

import (
	"reflect"
	"sync"
	"time"
)

// TypeInfo holds cached type information.
type TypeInfo struct {
	Type       reflect.Type
	Kind       reflect.Kind
	IsPtr      bool
	IsSlice    bool
	IsMap      bool
	IsStruct   bool
	IsTime     bool
	ElemType   reflect.Type
	KeyType    reflect.Type
	NumField   int
	Fields     []FieldInfo
	IsExported bool
}

// FieldInfo holds cached field information.
type FieldInfo struct {
	Index      []int
	Name       string
	ConfigName string
	Type       reflect.Type
	Kind       reflect.Kind
	IsExported bool
	HasTag     bool
	TagValue   string
}

// Cache caches type information.
type Cache struct {
	types  sync.Map
	fields sync.Map
}

// NewCache creates a new type cache.
func NewCache() *Cache {
	return &Cache{}
}

// GetTypeInfo returns cached type information.
func (c *Cache) GetTypeInfo(t reflect.Type) *TypeInfo {
	if cached, ok := c.types.Load(t); ok {
		return cached.(*TypeInfo)
	}

	info := c.buildTypeInfo(t)
	c.types.Store(t, info)

	return info
}

// buildTypeInfo builds type information.
func (c *Cache) buildTypeInfo(t reflect.Type) *TypeInfo {
	info := &TypeInfo{
		Type: t,
		Kind: t.Kind(),
	}

	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		info.IsPtr = true
		t = t.Elem()
		info.ElemType = t
		info.Kind = t.Kind()
	}

	info.IsSlice = info.Kind == reflect.Slice
	info.IsMap = info.Kind == reflect.Map
	info.IsStruct = info.Kind == reflect.Struct
	info.IsTime = t == reflect.TypeOf(time.Time{})

	if info.IsSlice {
		info.ElemType = t.Elem()
	}

	if info.IsMap {
		info.KeyType = t.Key()
		info.ElemType = t.Elem()
	}

	if info.IsStruct && !info.IsTime {
		info.NumField = t.NumField()
		info.Fields = c.buildFieldInfos(t)
	}

	info.IsExported = isExported(t)

	return info
}

// buildFieldInfos builds field information for a struct type.
func (c *Cache) buildFieldInfos(t reflect.Type) []FieldInfo {
	fields := make([]FieldInfo, 0, t.NumField())

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if field.PkgPath != "" {
			continue
		}

		fInfo := FieldInfo{
			Index:      field.Index,
			Name:       field.Name,
			Type:       field.Type,
			Kind:       field.Type.Kind(),
			IsExported: field.PkgPath == "",
		}

		// Parse config tag
		if tag := field.Tag.Get("config"); tag != "" {
			fInfo.HasTag = true
			fInfo.TagValue = tag
			fInfo.ConfigName = parseConfigTag(tag)
		} else {
			fInfo.ConfigName = toSnakeCase(field.Name)
		}

		fields = append(fields, fInfo)
	}

	return fields
}

// parseConfigTag parses a config tag value.
func parseConfigTag(tag string) string {
	// Handle tags like "name,required,default=value"
	for i := range len(tag) {
		if tag[i] == ',' {
			return tag[:i]
		}
	}

	return tag
}

// toSnakeCase converts CamelCase to snake_case.
func toSnakeCase(s string) string {
	result := make([]byte, 0, len(s)*2)

	for i := range len(s) {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				result = append(result, '_')
			}
			result = append(result, c+('a'-'A'))
		} else {
			result = append(result, c)
		}
	}

	return string(result)
}

// isExported returns true if the type is exported.
func isExported(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() == reflect.Struct {
		for i := 0; i < t.NumField(); i++ {
			if t.Field(i).PkgPath == "" {
				return true
			}
		}
	}

	return true
}

// ValueSetter sets values using reflection.
type ValueSetter struct {
	cache *Cache
}

// NewValueSetter creates a new value setter.
func NewValueSetter() *ValueSetter {
	return &ValueSetter{
		cache: NewCache(),
	}
}

// Set sets a value using reflection.
func (s *ValueSetter) Set(target reflect.Value, value any) error {
	if target.Kind() == reflect.Ptr {
		if target.IsNil() {
			target.Set(reflect.New(target.Type().Elem()))
		}
		target = target.Elem()
	}

	srcValue := reflect.ValueOf(value)
	srcType := srcValue.Type()
	targetType := target.Type()

	// Direct assignment if types match
	if srcType.AssignableTo(targetType) {
		target.Set(srcValue)

		return nil
	}

	// Convert if possible
	if srcType.ConvertibleTo(targetType) {
		target.Set(srcValue.Convert(targetType))

		return nil
	}

	// Handle common conversions
	switch target.Kind() {
	case reflect.String:
		target.SetString(stringify(value))

		return nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if targetType == reflect.TypeOf(time.Duration(0)) {
			// Handle duration
			if d, ok := value.(time.Duration); ok {
				target.SetInt(int64(d))

				return nil
			}
		}
		if i, ok := toInt64(value); ok {
			target.SetInt(i)

			return nil
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if i, ok := toInt64(value); ok && i >= 0 {
			target.SetUint(uint64(i))

			return nil
		}

	case reflect.Float32, reflect.Float64:
		if f, ok := toFloat64(value); ok {
			target.SetFloat(f)

			return nil
		}

	case reflect.Bool:
		if b, ok := toBool(value); ok {
			target.SetBool(b)

			return nil
		}
	}

	return nil
}

// stringify converts a value to string.
func stringify(v any) string {
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	default:
		return ""
	}
}

// toInt64 converts a value to int64.
func toInt64(v any) (int64, bool) {
	switch i := v.(type) {
	case int:
		return int64(i), true
	case int8:
		return int64(i), true
	case int16:
		return int64(i), true
	case int32:
		return int64(i), true
	case int64:
		return i, true
	case uint:
		return int64(i), true
	case uint8:
		return int64(i), true
	case uint16:
		return int64(i), true
	case uint32:
		return int64(i), true
	case uint64:
		return int64(i), true
	case float32:
		return int64(i), true
	case float64:
		return int64(i), true
	default:
		return 0, false
	}
}

// toFloat64 converts a value to float64.
func toFloat64(v any) (float64, bool) {
	switch f := v.(type) {
	case float32:
		return float64(f), true
	case float64:
		return f, true
	case int:
		return float64(f), true
	case int64:
		return float64(f), true
	case uint:
		return float64(f), true
	case uint64:
		return float64(f), true
	default:
		return 0, false
	}
}

// toBool converts a value to bool.
func toBool(v any) (bool, bool) {
	switch b := v.(type) {
	case bool:
		return b, true
	default:
		return false, false
	}
}

// WalkStruct walks through struct fields.
func WalkStruct(v any, fn func(field FieldInfo, value reflect.Value) error) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Struct {
		return nil
	}

	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		field := rt.Field(i)
		if field.PkgPath != "" {
			continue
		}

		fInfo := FieldInfo{
			Index: field.Index,
			Name:  field.Name,
			Type:  field.Type,
			Kind:  field.Type.Kind(),
		}

		if err := fn(fInfo, rv.Field(i)); err != nil {
			return err
		}
	}

	return nil
}

// IsZero checks if a value is its zero value.
func IsZero(v any) bool {
	if v == nil {
		return true
	}

	return reflect.ValueOf(v).IsZero()
}

// IsNil checks if a value is nil.
func IsNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func, reflect.Interface:
		return rv.IsNil()
	default:
		return false
	}
}

// Dereference dereferences a pointer value.
func Dereference(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}

	return v
}

// MakeSlice creates a slice of the given type and length.
func MakeSlice(t reflect.Type, len, cap int) reflect.Value {
	return reflect.MakeSlice(reflect.SliceOf(t), len, cap)
}

// MakeMap creates a map of the given type.
func MakeMap(keyType, valueType reflect.Type) reflect.Value {
	return reflect.MakeMap(reflect.MapOf(keyType, valueType))
}
