package observe

import "github.com/os-gomod/go-config"

// Observer reacts to config changes.
type Observer interface {
	OnChange([]config.Change)
}

// ObserverFunc adapts a function.
type ObserverFunc func([]config.Change)

func (f ObserverFunc) OnChange(c []config.Change) { f(c) }
