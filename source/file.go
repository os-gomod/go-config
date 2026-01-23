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
	return m, nil
}

func (s *FileSource) WatchPaths() []string {
	return []string{s.path}
}
