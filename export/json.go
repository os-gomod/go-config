package export

import "encoding/json"

// JSONExporter exports as JSON.
type JSONExporter struct{}

func (JSONExporter) Export(m map[string]any) ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}
