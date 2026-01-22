package config

import (
	"github.com/os-gomod/go-config/bind"
	"github.com/os-gomod/go-config/hooks"
	"github.com/os-gomod/go-config/source"
	"github.com/os-gomod/go-config/validate"
)

// DevelopmentPreset returns a dev-friendly builder.
func DevelopmentPreset() *Builder {
	return NewBuilder().
		WithHook(hooks.LoggingHook{}).
		WithSource(source.NewEnvSource("DEV_", 50))
}

// ProductionPreset returns a prod-ready builder.
func ProductionPreset() *Builder {
	return NewBuilder().
		WithHook(hooks.LoggingHook{}).
		WithHook(&hooks.TimingHook{})
}

// Bind binds config to struct.
func Bind(data map[string]any, dst any) error {
	return bind.Bind(data, dst)
}

// BindAndValidate binds config to struct and validates it.
func BindAndValidate(data map[string]any, dst any, validator *validate.Manager) error {
	return bind.BindAndValidate(data, dst, validator)
}
