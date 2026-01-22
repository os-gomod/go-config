package process

import "github.com/os-gomod/go-config/source"

// Middleware wraps a Source.
type Middleware func(source.Source) source.Source

// Chain applies middleware in reverse order (last middleware wraps first).
func Chain(mw ...Middleware) Middleware {
	return func(s source.Source) source.Source {
		return applyMiddleware(s, mw)
	}
}

func applyMiddleware(s source.Source, mw []Middleware) source.Source {
	for i := len(mw) - 1; i >= 0; i-- {
		s = mw[i](s)
	}
	return s
}
