package source

// Source is a configuration input.
type Source interface {
	Name() string
	Priority() int
	Load() (map[string]any, error)
	WatchPaths() []string
}

// BaseSource provides common fields.
type BaseSource struct {
	name     string
	priority int
}

func NewBase(name string, priority int) BaseSource {
	return BaseSource{name: name, priority: priority}
}

func (b BaseSource) Name() string         { return b.name }
func (b BaseSource) Priority() int        { return b.priority }
func (b BaseSource) WatchPaths() []string { return nil }

// Shared utility functions for sources
func copySourceData(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func mergeInto(dst, src map[string]any) {
	for k, v := range src {
		dst[k] = v
	}
}

func collectWatchPaths(sources []Source) []string {
	var paths []string
	for _, src := range sources {
		paths = append(paths, src.WatchPaths()...)
	}
	return paths
}
