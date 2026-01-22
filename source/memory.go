package source

// MemorySource stores static data.
type MemorySource struct {
	BaseSource
	data map[string]any
}

func NewMemorySource(data map[string]any, priority int) *MemorySource {
	return &MemorySource{
		BaseSource: NewBase("memory", priority),
		data:       data,
	}
}

func (s *MemorySource) Load() (map[string]any, error) {
	return copySourceData(s.data), nil
}
