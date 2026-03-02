// Package parser provides configuration file parsing utilities.
// Supports YAML, JSON, and TOML formats with zero-allocation where possible.
package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/os-gomod/go-config/types"
)

// Format represents a configuration file format.
type Format string

const (
	FormatYAML Format = "yaml"
	FormatJSON Format = "json"
	FormatTOML Format = "toml"
	FormatAuto Format = "auto" // Auto-detect from extension
)

// Parser parses configuration data from various formats.
type Parser interface {
	// Parse parses raw bytes into a map structure.
	Parse(data []byte) (map[string]any, error)

	// Format returns the parser format.
	Format() Format
}

// Registry holds registered parsers.
type Registry struct {
	parsers map[Format]Parser
	mu      sync.RWMutex
}

// NewRegistry creates a new parser registry with default parsers.
func NewRegistry() *Registry {
	r := &Registry{
		parsers: make(map[Format]Parser),
	}

	// Register default parsers
	r.Register(&yamlParser{})
	r.Register(&jsonParser{})
	r.Register(&tomlParser{})

	return r
}

// Register adds a parser to the registry.
func (r *Registry) Register(p Parser) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.parsers[p.Format()] = p
}

// Get retrieves a parser by format.
func (r *Registry) Get(format Format) (Parser, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.parsers[format]
	if !ok {
		return nil, types.NewError(types.ErrParseError,
			fmt.Sprintf("no parser registered for format: %s", format))
	}

	return p, nil
}

// DetectFormat determines format from file extension.
func DetectFormat(filename string) Format {
	ext := strings.ToLower(filename)

	// Handle compound extensions like .yaml.example
	if idx := strings.LastIndex(ext, "."); idx > 0 {
		ext = ext[idx:]
	}

	switch ext {
	case ".yaml", ".yml":
		return FormatYAML
	case ".json":
		return FormatJSON
	case ".toml":
		return FormatTOML
	default:
		return FormatAuto
	}
}

// yamlParser parses YAML format.
type yamlParser struct{}

func (p *yamlParser) Format() Format { return FormatYAML }

// yamlScanner provides a simple YAML parser without external dependencies.
// This is a minimal implementation for common YAML patterns.
func (p *yamlParser) Parse(data []byte) (map[string]any, error) {
	result := make(map[string]any)

	lines := bytes.Split(data, []byte("\n"))

	var currentKey string
	currentMap := result
	var mapStack []map[string]any
	var indentStack []int

	for _, line := range lines {
		// Skip empty lines and comments
		line = bytes.TrimSpace(line)
		if len(line) == 0 || line[0] == '#' {
			continue
		}

		// Calculate indentation
		indent := 0
		for i := range len(line) {
			if line[i] == ' ' || line[i] == '\t' {
				indent++
			} else {
				break
			}
		}

		// Handle indentation changes
		for len(indentStack) > 0 && indent <= indentStack[len(indentStack)-1] {
			indentStack = indentStack[:len(indentStack)-1]
			if len(mapStack) > 0 {
				currentMap = mapStack[len(mapStack)-1]
				mapStack = mapStack[:len(mapStack)-1]
			}
		}

		// Parse key-value pair
		line = bytes.TrimSpace(line)
		colonIdx := bytes.Index(line, []byte(":"))
		if colonIdx == -1 {
			continue
		}

		key := bytes.TrimSpace(line[:colonIdx])
		value := bytes.TrimSpace(line[colonIdx+1:])

		// Remove quotes from key
		key = bytes.Trim(key, "\"'")

		currentKey = string(key)

		if len(value) == 0 {
			// Nested map
			newMap := make(map[string]any)
			currentMap[currentKey] = newMap
			mapStack = append(mapStack, currentMap)
			indentStack = append(indentStack, indent)
			currentMap = newMap
		} else {
			// Leaf value
			currentMap[currentKey] = parseValue(string(value))
		}
	}

	return result, nil
}

// parseValue converts a YAML value string to appropriate type.
func parseValue(s string) any {
	s = strings.TrimSpace(s)

	// Remove quotes
	if (strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) ||
		(strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) {
		return s[1 : len(s)-1]
	}

	// Boolean
	switch strings.ToLower(s) {
	case "true", "yes", "on":
		return true
	case "false", "no", "off":
		return false
	}

	// Null
	if strings.EqualFold(s, "null") || s == "~" {
		return nil
	}

	// Integer
	var intVal int
	if _, err := fmt.Sscanf(s, "%d", &intVal); err == nil {
		return intVal
	}

	// Float
	var floatVal float64
	if _, err := fmt.Sscanf(s, "%f", &floatVal); err == nil {
		return floatVal
	}

	// String
	return s
}

// jsonParser parses JSON format.
type jsonParser struct{}

func (p *jsonParser) Format() Format { return FormatJSON }

func (p *jsonParser) Parse(data []byte) (map[string]any, error) {
	var result map[string]any

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber() // Preserve number precision

	if err := decoder.Decode(&result); err != nil {
		return nil, types.NewError(types.ErrParseError, "failed to parse JSON",
			types.WithCause(err))
	}

	converted := convertJSONNumbers(result)

	// Convert json.Number to appropriate types
	m, ok := converted.(map[string]any)
	if !ok {
		return nil, types.NewError(
			types.ErrParseError,
			"invalid JSON root type after conversion",
		)
	}

	return m, nil
}

// convertJSONNumbers recursively converts json.Number values.
func convertJSONNumbers(v any) any {
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, v := range val {
			result[k] = convertJSONNumbers(v)
		}

		return result
	case []any:
		result := make([]any, len(val))
		for i, v := range val {
			result[i] = convertJSONNumbers(v)
		}

		return result
	case json.Number:
		// Try int first, then float
		if i, err := val.Int64(); err == nil {
			return int(i)
		}
		if f, err := val.Float64(); err == nil {
			return f
		}

		return val.String()
	default:
		return v
	}
}

// tomlParser parses TOML format.
type tomlParser struct{}

func (p *tomlParser) Format() Format { return FormatTOML }

func (p *tomlParser) Parse(data []byte) (map[string]any, error) {
	// Simple TOML parser for basic key-value pairs
	result := make(map[string]any)

	lines := bytes.Split(data, []byte("\n"))
	sectionMap := result

	for _, line := range lines {
		line = bytes.TrimSpace(line)

		// Skip empty lines and comments
		if len(line) == 0 || line[0] == '#' {
			continue
		}

		// Section header
		if line[0] == '[' && line[len(line)-1] == ']' {
			section := string(line[1 : len(line)-1])

			// Create nested map for section
			parts := strings.Split(section, ".")
			sectionMap = result
			for _, part := range parts {
				if _, ok := sectionMap[part]; !ok {
					sectionMap[part] = make(map[string]any)
				}
				if m, ok := sectionMap[part].(map[string]any); ok {
					sectionMap = m
				}
			}

			continue
		}

		// Key-value pair
		kv := bytes.SplitN(line, []byte("="), 2)
		if len(kv) != 2 {
			continue
		}

		key := bytes.TrimSpace(kv[0])
		value := bytes.TrimSpace(kv[1])

		// Parse value
		sectionMap[string(key)] = parseTOMLValue(string(value))
	}

	return result, nil
}

// parseTOMLValue converts a TOML value string to appropriate type.
func parseTOMLValue(s string) any {
	s = strings.TrimSpace(s)

	// String (quoted)
	if (strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) ||
		(strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) {
		return s[1 : len(s)-1]
	}

	// Boolean
	switch strings.ToLower(s) {
	case "true":
		return true
	case "false":
		return false
	}

	// Integer
	var intVal int
	if _, err := fmt.Sscanf(s, "%d", &intVal); err == nil {
		return intVal
	}

	// Float
	var floatVal float64
	if _, err := fmt.Sscanf(s, "%f", &floatVal); err == nil {
		return floatVal
	}

	return s
}

// Flatten converts a nested map to dotted keys.
func Flatten(m map[string]any, prefix string) map[string]any {
	result := make(map[string]any)
	flattenRecursive(m, prefix, result)

	return result
}

func flattenRecursive(m map[string]any, prefix string, result map[string]any) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}

		switch val := v.(type) {
		case map[string]any:
			flattenRecursive(val, key, result)
		default:
			result[key] = v
		}
	}
}
