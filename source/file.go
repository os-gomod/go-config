package source

import (
	"os"

	"gopkg.in/yaml.v3"
)

// FileSource loads config from a YAML file.
type FileSource struct {
	BaseSource
	path string
}

func NewFileSource(path string, priority int) *FileSource {
	return &FileSource{
		BaseSource: NewBase("file:"+path, priority),
		path:       path,
	}
}

func (s *FileSource) Load() (map[string]any, error) {
	b, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}

	var m map[string]any
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, err
	}

	// Flatten nested structure into dot-notation keys
	return flatten(m), nil
}

func (s *FileSource) WatchPaths() []string {
	return []string{s.path}
}

// flatten converts nested maps into flat dot-notation keys
func flatten(m map[string]any) map[string]any {
	result := make(map[string]any)
	flattenHelper("", m, result)
	return result
}

func flattenHelper(prefix string, m map[string]any, result map[string]any) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}

		switch nested := v.(type) {
		case map[string]any:
			flattenHelper(key, nested, result)
		default:
			result[key] = v
		}
	}
}
