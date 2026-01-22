package process

// Processor mutates configuration data.
type Processor interface {
	Process(map[string]any) (map[string]any, error)
}
