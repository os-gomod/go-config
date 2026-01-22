package process

import "github.com/os-gomod/go-config/source"

// processingSource applies a Processor to a Source.
type processingSource struct {
	source source.Source
	proc   Processor
}

// NewProcessingSource wraps a source with a processor.
func NewProcessingSource(s source.Source, p Processor) source.Source {
	return &processingSource{source: s, proc: p}
}

func (p *processingSource) Name() string {
	return p.source.Name()
}

func (p *processingSource) Priority() int {
	return p.source.Priority()
}

func (p *processingSource) WatchPaths() []string {
	return p.source.WatchPaths()
}

func (p *processingSource) Load() (map[string]any, error) {
	data, err := p.source.Load()
	if err != nil {
		return nil, err
	}
	return p.proc.Process(data)
}
