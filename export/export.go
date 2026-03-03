// Package export provides configuration export utilities.
// Supports JSON, YAML, and other formats.
package export

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/os-gomod/go-config/types"
)

// Format represents an export format.
type Format string

const (
	FormatJSON Format = "json"
	FormatYAML Format = "yaml"
	FormatTOML Format = "toml"
	FormatEnv  Format = "env"
)

// Exporter exports configuration to various formats.
type Exporter interface {
	Export(data map[string]types.Value, w io.Writer) error
	Format() Format
}

// Registry holds registered exporters.
type Registry struct {
	exporters map[Format]Exporter
}

// NewRegistry creates a new exporter registry.
func NewRegistry() *Registry {
	r := &Registry{
		exporters: make(map[Format]Exporter),
	}

	r.Register(&jsonExporter{})
	r.Register(&yamlExporter{})
	r.Register(&tomlExporter{})
	r.Register(&envExporter{})

	return r
}

// Register adds an exporter to the registry.
func (r *Registry) Register(e Exporter) {
	r.exporters[e.Format()] = e
}

// Get retrieves an exporter by format.
func (r *Registry) Get(format Format) (Exporter, error) {
	e, ok := r.exporters[format]
	if !ok {
		return nil, types.NewError(types.ErrInvalidFormat,
			fmt.Sprintf("no exporter for format: %s", format))
	}

	return e, nil
}

// Export exports configuration to the specified format.
func (r *Registry) Export(data map[string]types.Value, w io.Writer, format Format) error {
	e, err := r.Get(format)
	if err != nil {
		return err
	}

	return e.Export(data, w)
}

// jsonExporter exports to JSON format.
type jsonExporter struct{}

func (e *jsonExporter) Format() Format { return FormatJSON }

func (e *jsonExporter) Export(data map[string]types.Value, w io.Writer) error {
	// Build nested structure
	nested := buildNested(data)

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)

	return encoder.Encode(nested)
}

// yamlExporter exports to YAML format.
type yamlExporter struct{}

func (e *yamlExporter) Format() Format { return FormatYAML }

func (e *yamlExporter) Export(data map[string]types.Value, w io.Writer) error {
	// Build nested structure
	nested := buildNested(data)

	// Simple YAML serializer
	return writeYAML(w, nested, 0)
}

// writeYAML writes a value as YAML.
func writeYAML(w io.Writer, v any, indent int) error {
	switch val := v.(type) {
	case map[string]any:
		return writeYAMLMap(w, val, indent)
	case []any:
		return writeYAMLArray(w, val, indent)
	case string:
		// Quote strings with special characters
		if needsQuoting(val) {
			_, err := fmt.Fprintf(w, "\"%s\"\n", escapeString(val))

			return err
		}
		_, err := fmt.Fprintf(w, "%s\n", val)

		return err
	case int:
		_, err := fmt.Fprintf(w, "%d\n", val)

		return err
	case int64:
		_, err := fmt.Fprintf(w, "%d\n", val)

		return err
	case float64:
		_, err := fmt.Fprintf(w, "%v\n", val)

		return err
	case bool:
		if val {
			_, err := w.Write([]byte("true\n"))

			return err
		}
		_, err := w.Write([]byte("false\n"))

		return err
	case time.Duration:
		_, err := fmt.Fprintf(w, "\"%s\"\n", val.String())

		return err
	case time.Time:
		_, err := fmt.Fprintf(w, "\"%s\"\n", val.Format(time.RFC3339))

		return err
	case nil:
		_, err := w.Write([]byte("null\n"))

		return err
	default:
		_, err := fmt.Fprintf(w, "%v\n", val)

		return err
	}
}

// writeYAMLMap writes a map as YAML.
func writeYAMLMap(w io.Writer, m map[string]any, indent int) error {
	// Sort keys for deterministic output
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	indentStr := strings.Repeat("  ", indent)
	var err error

	for _, key := range keys {
		value := m[key]

		// Write key
		_, err = fmt.Fprintf(w, "%s%s:", indentStr, key)
		if err != nil {
			return err
		}

		// Write value
		switch v := value.(type) {
		case map[string]any:
			_, err = w.Write([]byte("\n"))
			if err != nil {
				return err
			}
			err = writeYAMLMap(w, v, indent+1)
			if err != nil {
				return err
			}
		case []any:
			_, err = w.Write([]byte("\n"))
			if err != nil {
				return err
			}
			err = writeYAMLArray(w, v, indent+1)
			if err != nil {
				return err
			}
		default:
			_, err = w.Write([]byte(" "))
			if err != nil {
				return err
			}
			err = writeYAML(w, v, indent)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// writeYAMLArray writes an array as YAML.
//
//nolint:gocyclo // Nested YAML structures require multi-branch handling.
func writeYAMLArray(w io.Writer, a []any, indent int) error {
	indentStr := strings.Repeat("  ", indent)
	var err error

	for _, value := range a {
		_, err = fmt.Fprintf(w, "%s-", indentStr)
		if err != nil {
			return err
		}

		switch v := value.(type) {
		case map[string]any:
			if len(v) == 0 {
				_, err = w.Write([]byte(" {}\n"))
				if err != nil {
					return err
				}
			} else {
				_, err = w.Write([]byte("\n"))
				if err != nil {
					return err
				}
				// First key on same line as dash
				keys := make([]string, 0, len(v))
				for k := range v {
					keys = append(keys, k)
				}
				sort.Strings(keys)

				for _, key := range keys {
					_, err = fmt.Fprintf(w, "%s  %s:", indentStr, key)
					if err != nil {
						return err
					}

					switch vv := v[key].(type) {
					case map[string]any:
						_, err = w.Write([]byte("\n"))
						if err != nil {
							return err
						}
						err = writeYAMLMap(w, vv, indent+2)
						if err != nil {
							return err
						}
					default:
						_, err = w.Write([]byte(" "))
						if err != nil {
							return err
						}
						err = writeYAML(w, vv, indent)
						if err != nil {
							return err
						}
					}
				}
			}
		case []any:
			_, err = w.Write([]byte("\n"))
			if err != nil {
				return err
			}
			err = writeYAMLArray(w, v, indent+1)
			if err != nil {
				return err
			}
		default:
			_, err = w.Write([]byte(" "))
			if err != nil {
				return err
			}
			err = writeYAML(w, v, indent)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// tomlExporter exports to TOML format.
type tomlExporter struct{}

func (e *tomlExporter) Format() Format { return FormatTOML }

func (e *tomlExporter) Export(data map[string]types.Value, w io.Writer) error {
	// Build nested structure
	nested := buildNested(data)

	return writeTOML(w, nested, "")
}

// writeTOML writes a value as TOML.
func writeTOML(w io.Writer, v any, prefix string) error {
	switch val := v.(type) {
	case map[string]any:
		return writeTOMLMap(w, val, prefix)
	default:
		return nil
	}
}

// writeTOMLMap writes a map as TOML.
func writeTOMLMap(w io.Writer, m map[string]any, prefix string) error {
	// Sort keys
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var err error
	for _, key := range keys {
		value := m[key]
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		switch v := value.(type) {
		case map[string]any:
			// Check if nested map contains only primitive values
			if isFlatMap(v) {
				// Write as dotted keys
				for k2, v2 := range v {
					err = writeTOMLKey(w, fullKey+"."+k2, v2)
					if err != nil {
						return err
					}
				}
			} else {
				// Write as section
				_, err = fmt.Fprintf(w, "[%s]\n", fullKey)
				if err != nil {
					return err
				}
				err = writeTOML(w, v, fullKey)
				if err != nil {
					return err
				}
			}
		default:
			err = writeTOMLKey(w, fullKey, v)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// writeTOMLKey writes a key-value pair as TOML.
func writeTOMLKey(w io.Writer, key string, value any) error {
	switch v := value.(type) {
	case string:
		_, err := fmt.Fprintf(w, "%s = \"%s\"\n", key, escapeString(v))

		return err
	case int:
		_, err := fmt.Fprintf(w, "%s = %d\n", key, v)

		return err
	case int64:
		_, err := fmt.Fprintf(w, "%s = %d\n", key, v)

		return err
	case float64:
		_, err := fmt.Fprintf(w, "%s = %v\n", key, v)

		return err
	case bool:
		if v {
			_, err := fmt.Fprintf(w, "%s = true\n", key)

			return err
		}
		_, err := fmt.Fprintf(w, "%s = false\n", key)

		return err
	case time.Duration:
		_, err := fmt.Fprintf(w, "%s = \"%s\"\n", key, v.String())

		return err
	case time.Time:
		_, err := fmt.Fprintf(w, "%s = \"%s\"\n", key, v.Format(time.RFC3339))

		return err
	case []any:
		return writeTOMLArray(w, key, v)
	default:
		_, err := fmt.Fprintf(w, "%s = %v\n", key, v)

		return err
	}
}

// writeTOMLArray writes an array as TOML.
func writeTOMLArray(w io.Writer, key string, a []any) error {
	_, err := fmt.Fprintf(w, "%s = [", key)
	if err != nil {
		return err
	}

	for i, v := range a {
		if i > 0 {
			_, err = w.Write([]byte(", "))
			if err != nil {
				return err
			}
		}

		switch vv := v.(type) {
		case string:
			_, err = fmt.Fprintf(w, "\"%s\"", escapeString(vv))
			if err != nil {
				return err
			}
		case int:
			_, err = fmt.Fprintf(w, "%d", vv)
			if err != nil {
				return err
			}
		case bool:
			if vv {
				_, err = w.Write([]byte("true"))
				if err != nil {
					return err
				}
			} else {
				_, err = w.Write([]byte("false"))
				if err != nil {
					return err
				}
			}
		default:
			_, err = fmt.Fprintf(w, "%v", vv)
			if err != nil {
				return err
			}
		}
	}

	_, err = w.Write([]byte("]\n"))

	return err
}

// isFlatMap checks if a map contains only primitive values.
func isFlatMap(m map[string]any) bool {
	for _, v := range m {
		switch v.(type) {
		case map[string]any, []any:
			return false
		}
	}

	return true
}

// envExporter exports to environment variable format.
type envExporter struct{}

func (e *envExporter) Format() Format { return FormatEnv }

func (e *envExporter) Export(data map[string]types.Value, w io.Writer) error {
	// Sort keys for deterministic output
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := data[key]
		envKey := strings.ToUpper(strings.ReplaceAll(key, ".", "_"))

		var strValue string
		switch v := value.Raw().(type) {
		case string:
			strValue = v
		case int, int64, float64:
			strValue = fmt.Sprintf("%v", v)
		case bool:
			strValue = strconv.FormatBool(v)
		case time.Duration:
			strValue = v.String()
		case time.Time:
			strValue = v.Format(time.RFC3339)
		default:
			strValue = fmt.Sprintf("%v", v)
		}

		_, err := fmt.Fprintf(w, "%s=%s\n", envKey, strValue)
		if err != nil {
			return err
		}
	}

	return nil
}

// buildNested converts flattened keys to nested structure.
func buildNested(data map[string]types.Value) map[string]any {
	result := make(map[string]any)

	for key, value := range data {
		parts := strings.Split(key, ".")
		current := result

		for i, part := range parts {
			if i == len(parts)-1 {
				current[part] = value.Raw()
			} else {
				if _, ok := current[part]; !ok {
					current[part] = make(map[string]any)
				}
				if next, ok := current[part].(map[string]any); ok {
					current = next
				}
			}
		}
	}

	return result
}

// needsQuoting checks if a YAML string needs quoting.
//
//nolint:gocyclo // Explicit character checks are intentional for fast path behavior.
func needsQuoting(s string) bool {
	if s == "" {
		return true
	}

	// Check for special characters
	for _, r := range s {
		if r == ':' || r == '#' || r == '{' || r == '}' || r == '[' || r == ']' ||
			r == ',' || r == '&' || r == '*' || r == '?' || r == '|' || r == '-' ||
			r == '<' || r == '>' || r == '=' || r == '!' || r == '%' || r == '@' {
			return true
		}
	}

	// Check for boolean/null values
	switch s {
	case "true", "false", "null", "yes", "no", "on", "off", "~":
		return true
	}

	// Check for numbers
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return true
	}

	// Check for leading/trailing whitespace
	if s != "" && (s[0] == ' ' || s[len(s)-1] == ' ' || s[0] == '\t' || s[len(s)-1] == '\t') {
		return true
	}

	return false
}

// escapeString escapes special characters in a string.
func escapeString(s string) string {
	var buf bytes.Buffer
	for _, r := range s {
		switch r {
		case '"':
			buf.WriteString("\\\"")
		case '\\':
			buf.WriteString("\\\\")
		case '\n':
			buf.WriteString("\\n")
		case '\r':
			buf.WriteString("\\r")
		case '\t':
			buf.WriteString("\\t")
		default:
			buf.WriteRune(r)
		}
	}

	return buf.String()
}

// ToJSON exports configuration to JSON format.
func ToJSON(data map[string]types.Value) ([]byte, error) {
	var buf bytes.Buffer
	registry := NewRegistry()
	err := registry.Export(data, &buf, FormatJSON)

	return buf.Bytes(), err
}

// ToYAML exports configuration to YAML format.
func ToYAML(data map[string]types.Value) ([]byte, error) {
	var buf bytes.Buffer
	registry := NewRegistry()
	err := registry.Export(data, &buf, FormatYAML)

	return buf.Bytes(), err
}

// ToTOML exports configuration to TOML format.
func ToTOML(data map[string]types.Value) ([]byte, error) {
	var buf bytes.Buffer
	registry := NewRegistry()
	err := registry.Export(data, &buf, FormatTOML)

	return buf.Bytes(), err
}

// ToEnv exports configuration to environment variable format.
func ToEnv(data map[string]types.Value) ([]byte, error) {
	var buf bytes.Buffer
	registry := NewRegistry()
	err := registry.Export(data, &buf, FormatEnv)

	return buf.Bytes(), err
}
