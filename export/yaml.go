package export

import "gopkg.in/yaml.v3"

// YAMLExporter exports as YAML.
type YAMLExporter struct{}

func (YAMLExporter) Export(m map[string]any) ([]byte, error) {
	return yaml.Marshal(m)
}
