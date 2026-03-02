// Package bind provides type-safe struct binding for configuration values.
// Uses cached reflection metadata to minimize runtime overhead.
package bind

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/os-gomod/go-config/types"
)

// Binder binds configuration values to structs.
type Binder struct {
	cache   sync.Map // Cache of struct metadata
	tagName string   // Struct tag name for configuration
}

// NewBinder creates a new binder.
func NewBinder() *Binder {
	return &Binder{
		tagName: "config",
	}
}

// Bind binds configuration values to a struct.
func (b *Binder) Bind(data map[string]types.Value, target any) error {
	if target == nil {
		return types.NewError(types.ErrBindError, "target cannot be nil")
	}

	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Ptr {
		return types.NewError(types.ErrBindError, "target must be a pointer")
	}

	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return types.NewError(types.ErrBindError, "target must be a pointer to struct")
	}

	rt := rv.Type()

	// Get or create metadata
	meta := b.getMeta(rt)

	// Bind fields
	for _, field := range meta.fields {
		fv := rv.FieldByIndex(field.index)

		if !fv.CanSet() {
			continue
		}

		// Get value from config
		value, exists := data[field.key]
		if !exists && field.hasDefault {
			value = types.NewValue(field.defaultValue, types.TypeString, types.SourceDefault, -1)
		}

		if !exists && !field.hasDefault {
			if field.required {
				return types.NewError(types.ErrBindError,
					fmt.Sprintf("required field '%s' not found", field.key))
			}

			continue
		}

		if err := b.setValue(fv, value, field); err != nil {
			return err
		}
	}

	return nil
}

// setValue sets a field value from a configuration value.
func (b *Binder) setValue(fv reflect.Value, value types.Value, field *fieldMeta) error {
	// Handle pointer types
	if fv.Kind() == reflect.Ptr {
		if fv.IsNil() {
			fv.Set(reflect.New(fv.Type().Elem()))
		}
		fv = fv.Elem()
	}

	// Handle different types
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(value.String())

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// Check for duration type
		if fv.Type() == reflect.TypeOf(time.Duration(0)) {
			if d, ok := value.Duration(); ok {
				fv.SetInt(int64(d))
			} else {
				d, err := time.ParseDuration(value.String())
				if err != nil {
					return types.NewError(types.ErrTypeMismatch,
						fmt.Sprintf("invalid duration for field '%s'", field.key),
						types.WithCause(err))
				}
				fv.SetInt(int64(d))
			}
		} else {
			if i, ok := value.Int(); ok {
				fv.SetInt(int64(i))
			} else {
				i, err := strconv.ParseInt(value.String(), 10, 64)
				if err != nil {
					return types.NewError(types.ErrTypeMismatch,
						fmt.Sprintf("invalid int for field '%s'", field.key),
						types.WithCause(err))
				}
				fv.SetInt(i)
			}
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if i, ok := value.Int(); ok {
			fv.SetUint(uint64(i))
		} else {
			i, err := strconv.ParseUint(value.String(), 10, 64)
			if err != nil {
				return types.NewError(types.ErrTypeMismatch,
					fmt.Sprintf("invalid uint for field '%s'", field.key),
					types.WithCause(err))
			}
			fv.SetUint(i)
		}

	case reflect.Float32, reflect.Float64:
		if f, ok := value.Float64(); ok {
			fv.SetFloat(f)
		} else {
			f, err := strconv.ParseFloat(value.String(), 64)
			if err != nil {
				return types.NewError(types.ErrTypeMismatch,
					fmt.Sprintf("invalid float for field '%s'", field.key),
					types.WithCause(err))
			}
			fv.SetFloat(f)
		}

	case reflect.Bool:
		if bl, ok := value.Bool(); ok {
			fv.SetBool(bl)
		} else {
			bl, err := strconv.ParseBool(value.String())
			if err != nil {
				return types.NewError(types.ErrTypeMismatch,
					fmt.Sprintf("invalid bool for field '%s'", field.key),
					types.WithCause(err))
			}
			fv.SetBool(bl)
		}

	case reflect.Slice:
		return b.setSlice(fv, value, field)

	case reflect.Map:
		return b.setMap(fv, value, field)

	case reflect.Struct:
		return b.setStruct(fv, value, field)

	default:
		return types.NewError(types.ErrTypeMismatch,
			fmt.Sprintf("unsupported type for field '%s': %s", field.key, fv.Kind()))
	}

	return nil
}

// setSlice sets a slice field from a configuration value.
func (b *Binder) setSlice(fv reflect.Value, value types.Value, field *fieldMeta) error {
	slice, ok := value.Raw().([]any)
	if !ok {
		return types.NewError(types.ErrTypeMismatch,
			fmt.Sprintf("expected slice for field '%s'", field.key))
	}

	elemType := fv.Type().Elem()
	sliceValue := reflect.MakeSlice(fv.Type(), len(slice), len(slice))

	for i, elem := range slice {
		ev := sliceValue.Index(i)
		if err := setReflectValue(ev, elem, elemType); err != nil {
			return types.NewError(types.ErrTypeMismatch,
				fmt.Sprintf("invalid slice element for field '%s[%d]'", field.key, i),
				types.WithCause(err))
		}
	}

	fv.Set(sliceValue)

	return nil
}

// setMap sets a map field from a configuration value.
func (b *Binder) setMap(fv reflect.Value, value types.Value, field *fieldMeta) error {
	m, ok := value.Raw().(map[string]any)
	if !ok {
		return types.NewError(types.ErrTypeMismatch,
			fmt.Sprintf("expected map for field '%s'", field.key))
	}

	keyType := fv.Type().Key()
	valueType := fv.Type().Elem()
	mapValue := reflect.MakeMap(fv.Type())

	for k, v := range m {
		key := reflect.ValueOf(k)
		if !key.Type().AssignableTo(keyType) {
			// Convert string key to keyType
			key = reflect.ValueOf(k).Convert(keyType)
		}

		val := reflect.New(valueType).Elem()
		if err := setReflectValue(val, v, valueType); err != nil {
			return types.NewError(types.ErrTypeMismatch,
				fmt.Sprintf("invalid map value for field '%s[%s]'", field.key, k),
				types.WithCause(err))
		}

		mapValue.SetMapIndex(key, val)
	}

	fv.Set(mapValue)

	return nil
}

// setStruct sets a struct field from a configuration value.
func (b *Binder) setStruct(fv reflect.Value, value types.Value, field *fieldMeta) error {
	// Check for time.Time
	if fv.Type() == reflect.TypeOf(time.Time{}) {
		t, err := time.Parse(time.RFC3339, value.String())
		if err != nil {
			return types.NewError(types.ErrTypeMismatch,
				fmt.Sprintf("invalid time for field '%s'", field.key),
				types.WithCause(err))
		}
		fv.Set(reflect.ValueOf(t))

		return nil
	}

	// Handle nested struct
	m, ok := value.Raw().(map[string]any)
	if !ok {
		return types.NewError(types.ErrTypeMismatch,
			fmt.Sprintf("expected map for struct field '%s'", field.key))
	}

	// Convert to typed values
	data := make(map[string]types.Value)
	for k, v := range m {
		data[k] = types.NewValue(v, types.TypeUnknown, types.SourceMemory, 0)
	}

	return b.Bind(data, fv.Addr().Interface())
}

// setReflectValue sets a reflect.Value from a raw value.
func setReflectValue(v reflect.Value, raw any, target reflect.Type) error {
	if raw == nil {
		return nil
	}

	switch target.Kind() {
	case reflect.String:
		v.SetString(fmt.Sprintf("%v", raw))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch r := raw.(type) {
		case int:
			v.SetInt(int64(r))
		case int64:
			v.SetInt(r)
		case float64:
			v.SetInt(int64(r))
		case string:
			i, err := strconv.ParseInt(r, 10, 64)
			if err != nil {
				return err
			}
			v.SetInt(i)
		}
	case reflect.Float32, reflect.Float64:
		switch r := raw.(type) {
		case float64:
			v.SetFloat(r)
		case int:
			v.SetFloat(float64(r))
		case string:
			f, err := strconv.ParseFloat(r, 64)
			if err != nil {
				return err
			}
			v.SetFloat(f)
		}
	case reflect.Bool:
		switch r := raw.(type) {
		case bool:
			v.SetBool(r)
		case string:
			b, err := strconv.ParseBool(r)
			if err != nil {
				return err
			}
			v.SetBool(b)
		}
	default:
		return fmt.Errorf("unsupported type: %s", target.Kind())
	}

	return nil
}

// structMeta holds cached metadata for a struct type.
type structMeta struct {
	fields []*fieldMeta
}

// fieldMeta holds metadata for a struct field.
type fieldMeta struct {
	index        []int
	key          string
	required     bool
	hasDefault   bool
	defaultValue string
}

// getMeta retrieves or creates metadata for a struct type.
func (b *Binder) getMeta(rt reflect.Type) *structMeta {
	if cached, ok := b.cache.Load(rt); ok {
		return cached.(*structMeta)
	}

	meta := b.buildMeta(rt)
	b.cache.Store(rt, meta)

	return meta
}

// buildMeta builds metadata for a struct type.
func (b *Binder) buildMeta(rt reflect.Type) *structMeta {
	meta := &structMeta{
		fields: make([]*fieldMeta, 0),
	}

	b.walkFields(rt, nil, nil, meta)

	return meta
}

// walkFields recursively walks struct fields.
func (b *Binder) walkFields(rt reflect.Type, prefix []string, parentIndex []int, meta *structMeta) {
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)

		// Skip unexported fields
		if field.PkgPath != "" {
			continue
		}

		// Skip embedded fields (handle separately if needed)
		if field.Anonymous {
			b.walkFields(field.Type, prefix, parentIndex, meta)

			continue
		}

		// Parse tag
		tag := field.Tag.Get(b.tagName)
		if tag == "-" {
			continue
		}

		fm := b.parseTag(field, tag, prefix, parentIndex)
		if fm != nil {
			meta.fields = append(meta.fields, fm)
		}

		// Recurse into nested structs
		if field.Type.Kind() == reflect.Struct &&
			field.Type != reflect.TypeOf(time.Time{}) {
			newPrefix := splitPath(fm.key)
			b.walkFields(field.Type, newPrefix, fm.index, meta)
		}
	}
}

// parseTag parses a struct tag into field metadata.
func (b *Binder) parseTag(field reflect.StructField, tag string, prefix []string, parentIndex []int) *fieldMeta {
	fm := &fieldMeta{
		index: append(append([]int(nil), parentIndex...), field.Index...),
	}

	// Get key from tag or field name
	if tag != "" {
		parts := strings.Split(tag, ",")
		fm.key = parts[0]

		for _, part := range parts[1:] {
			kv := strings.SplitN(part, "=", 2)
			switch kv[0] {
			case "required":
				fm.required = true
			case "default":
				if len(kv) > 1 {
					fm.hasDefault = true
					fm.defaultValue = kv[1]
				}
			}
		}
	}

	if fm.key == "" {
		fm.key = strings.ToLower(field.Name)
	}

	// Add prefix
	if len(prefix) > 0 {
		fm.key = strings.Join(prefix, ".") + "." + fm.key
	}

	return fm
}

func splitPath(key string) []string {
	if key == "" {
		return nil
	}
	parts := strings.Split(key, ".")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			out = append(out, part)
		}
	}

	return out
}

// MustBind binds configuration values to a struct, panicking on error.
func (b *Binder) MustBind(data map[string]types.Value, target any) {
	if err := b.Bind(data, target); err != nil {
		panic(err)
	}
}

// Schema generates a schema from a struct type.
func (b *Binder) Schema(target any) (*Schema, error) {
	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Ptr {
		return nil, types.NewError(types.ErrBindError, "target must be a pointer")
	}

	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return nil, types.NewError(types.ErrBindError, "target must be a pointer to struct")
	}

	rt := rv.Type()
	meta := b.getMeta(rt)

	return b.buildSchema(rt, meta), nil
}

// Schema represents a configuration schema.
type Schema struct {
	Type       string            `json:"type"`
	Properties map[string]*Field `json:"properties,omitempty"`
	Required   []string          `json:"required,omitempty"`
}

// Field represents a schema field.
type Field struct {
	Type        string            `json:"type"`
	Description string            `json:"description,omitempty"`
	Default     any               `json:"default,omitempty"`
	Required    bool              `json:"required,omitempty"`
	Properties  map[string]*Field `json:"properties,omitempty"`
	Items       *Field            `json:"items,omitempty"`
}

// buildSchema builds a schema from struct metadata.
func (b *Binder) buildSchema(rt reflect.Type, meta *structMeta) *Schema {
	schema := &Schema{
		Type:       "object",
		Properties: make(map[string]*Field),
		Required:   make([]string, 0),
	}

	for _, fm := range meta.fields {
		field := rt.FieldByIndex(fm.index)
		fieldType := b.fieldType(field.Type, fm)

		key := fm.key
		if idx := strings.LastIndex(key, "."); idx >= 0 {
			key = key[idx+1:]
		}

		schema.Properties[key] = fieldType

		if fm.required {
			schema.Required = append(schema.Required, key)
		}
	}

	return schema
}

// fieldType returns the schema type for a Go type.
func (b *Binder) fieldType(t reflect.Type, fm *fieldMeta) *Field {
	f := &Field{
		Required: fm.required,
	}

	if fm.hasDefault {
		f.Default = fm.defaultValue
	}

	switch t.Kind() {
	case reflect.String:
		f.Type = "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if t == reflect.TypeOf(time.Duration(0)) {
			f.Type = "string"
			f.Description = "duration string (e.g., '30s', '5m')"
		} else {
			f.Type = "integer"
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		f.Type = "integer"
	case reflect.Float32, reflect.Float64:
		f.Type = "number"
	case reflect.Bool:
		f.Type = "boolean"
	case reflect.Slice:
		f.Type = "array"
		f.Items = b.fieldType(t.Elem(), &fieldMeta{})
	case reflect.Map:
		f.Type = "object"
		f.Properties = make(map[string]*Field)
		f.Properties["*"] = b.fieldType(t.Elem(), &fieldMeta{})
	case reflect.Struct:
		if t == reflect.TypeOf(time.Time{}) {
			f.Type = "string"
			f.Description = "RFC3339 timestamp"
		} else {
			f.Type = "object"
			f.Properties = make(map[string]*Field)
			// Could recurse here for nested structs
		}
	case reflect.Ptr:
		return b.fieldType(t.Elem(), fm)
	default:
		f.Type = "any"
	}

	return f
}

// BindContext binds configuration values to a struct with context.
func (b *Binder) BindContext(ctx context.Context, data map[string]types.Value, target any) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return b.Bind(data, target)
	}
}
