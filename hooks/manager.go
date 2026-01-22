package hooks

import (
	"fmt"
	"sort"
)

// Manager executes lifecycle hooks.
type Manager struct {
	preLoad  []PreLoadHook
	postLoad []PostLoadHook
	preBind  []PreBindHook
	postBind []PostBindHook
}

func NewManager() *Manager {
	return &Manager{}
}

// Register detects and registers hook types.
func (m *Manager) Register(h Hook) {
	m.registerPreLoad(h)
	m.registerPostLoad(h)
	m.registerPreBind(h)
	m.registerPostBind(h)
}

func (m *Manager) registerPreLoad(h Hook) {
	if x, ok := h.(PreLoadHook); ok {
		m.preLoad = append(m.preLoad, x)
		sortHooks(m.preLoad)
	}
}

func (m *Manager) registerPostLoad(h Hook) {
	if x, ok := h.(PostLoadHook); ok {
		m.postLoad = append(m.postLoad, x)
		sortHooks(m.postLoad)
	}
}

func (m *Manager) registerPreBind(h Hook) {
	if x, ok := h.(PreBindHook); ok {
		m.preBind = append(m.preBind, x)
		sortHooks(m.preBind)
	}
}

func (m *Manager) registerPostBind(h Hook) {
	if x, ok := h.(PostBindHook); ok {
		m.postBind = append(m.postBind, x)
		sortHooks(m.postBind)
	}
}

func sortHooks[T Hook](h []T) {
	sort.Slice(h, func(i, j int) bool {
		return h[i].Priority() < h[j].Priority()
	})
}

// Run executes hooks for a stage.
func (m *Manager) Run(stage Stage, arg any) error {
	switch stage {
	case PreLoad:
		return m.runPreLoad(arg)
	case PostLoad:
		return m.runPostLoad(arg)
	case PreBind:
		return m.runPreBind(arg)
	case PostBind:
		return m.runPostBind(arg)
	}
	return nil
}

func (m *Manager) runPreLoad(arg any) error {
	return runHooksWithData(m.preLoad, arg, "pre-load",
		func(h PreLoadHook, data map[string]any) error {
			return h.OnPreLoad(data)
		})
}

func (m *Manager) runPostLoad(arg any) error {
	return runHooksWithData(m.postLoad, arg, "post-load",
		func(h PostLoadHook, data map[string]any) error {
			return h.OnPostLoad(data)
		})
}

func (m *Manager) runPreBind(arg any) error {
	return runHooksWithArg(m.preBind, arg, "pre-bind",
		func(h PreBindHook, a any) error {
			return h.OnPreBind(a)
		})
}

func (m *Manager) runPostBind(arg any) error {
	return runHooksWithArg(m.postBind, arg, "post-bind",
		func(h PostBindHook, a any) error {
			return h.OnPostBind(a)
		})
}

func runHooksWithData[T Hook](hooks []T, arg any, stage string, fn func(T, map[string]any) error) error {
	data := arg.(map[string]any)
	for _, h := range hooks {
		if err := fn(h, data); err != nil {
			return fmt.Errorf("%s %s: %w", stage, h.Name(), err)
		}
	}
	return nil
}

func runHooksWithArg[T Hook](hooks []T, arg any, stage string, fn func(T, any) error) error {
	for _, h := range hooks {
		if err := fn(h, arg); err != nil {
			return fmt.Errorf("%s %s: %w", stage, h.Name(), err)
		}
	}
	return nil
}
