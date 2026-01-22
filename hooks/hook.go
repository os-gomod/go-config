package hooks

// Stage represents a lifecycle stage.
type Stage int

const (
	PreLoad Stage = iota
	PostLoad
	PreBind
	PostBind
)

// Hook is the base hook interface.
type Hook interface {
	Name() string
	Priority() int // lower runs first
}

// PreLoadHook runs before loading.
type PreLoadHook interface {
	Hook
	OnPreLoad(data map[string]any) error
}

// PostLoadHook runs after loading.
type PostLoadHook interface {
	Hook
	OnPostLoad(data map[string]any) error
}

// PreBindHook runs before binding.
type PreBindHook interface {
	Hook
	OnPreBind(dst any) error
}

// PostBindHook runs after binding.
type PostBindHook interface {
	Hook
	OnPostBind(dst any) error
}
