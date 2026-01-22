package source

import "sort"

// CompositeSource merges multiple sources.
type CompositeSource struct {
	BaseSource
	sources []Source
}

func NewComposite(name string, priority int, sources ...Source) *CompositeSource {
	sortedSources := make([]Source, len(sources))
	copy(sortedSources, sources)

	sort.Slice(sortedSources, func(i, j int) bool {
		return sortedSources[i].Priority() < sortedSources[j].Priority()
	})

	return &CompositeSource{
		BaseSource: NewBase("composite:"+name, priority),
		sources:    sortedSources,
	}
}

func (s *CompositeSource) Load() (map[string]any, error) {
	merged := make(map[string]any)
	for _, src := range s.sources {
		m, err := src.Load()
		if err != nil {
			return nil, err
		}
		mergeInto(merged, m)
	}
	return merged, nil
}

func (s *CompositeSource) WatchPaths() []string {
	return collectWatchPaths(s.sources)
}
