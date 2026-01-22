package source

// ConditionalSource loads only when condition is true.
type ConditionalSource struct {
	BaseSource
	source    Source
	condition func() bool
}

func NewConditional(src Source, cond func() bool) *ConditionalSource {
	return &ConditionalSource{
		BaseSource: NewBase("conditional:"+src.Name(), src.Priority()),
		source:     src,
		condition:  cond,
	}
}

func (s *ConditionalSource) Load() (map[string]any, error) {
	if !s.condition() {
		return map[string]any{}, nil
	}
	return s.source.Load()
}

func (s *ConditionalSource) WatchPaths() []string {
	return s.source.WatchPaths()
}
