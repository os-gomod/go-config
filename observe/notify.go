package observe

import "github.com/os-gomod/go-config"

// Notify dispatches changes to observers.
func Notify(obs []config.Observer, c []config.Change) {
	for _, o := range obs {
		o.OnChange(c)
	}
}
