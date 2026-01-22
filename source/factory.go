package source

// Factory creates sources with a default priority.
type Factory struct {
	priority int
}

func NewFactory(priority int) *Factory {
	return &Factory{priority: priority}
}

func (f *Factory) Memory(data map[string]any) Source {
	return NewMemorySource(data, f.priority)
}

func (f *Factory) File(path string) Source {
	return NewFileSource(path, f.priority)
}

func (f *Factory) Env(prefix string) Source {
	return NewEnvSource(prefix, f.priority)
}
