package export

// Exporter serializes config data.
type Exporter interface {
	Export(map[string]any) ([]byte, error)
}
