package config

import (
	"fmt"

	"github.com/os-gomod/go-config/hooks"
	"github.com/os-gomod/go-config/process"
	"github.com/os-gomod/go-config/source"
	"github.com/os-gomod/go-config/validate"
)

// Builder assembles a Config instance.
type Builder struct {
	cfg        *Config
	sources    []source.Source
	middleware []process.Middleware
	hooks      *hooks.Manager
	validator  *validate.Manager
}

func NewBuilder() *Builder {
	return &Builder{
		cfg:        NewConfig(),
		sources:    []source.Source{},
		middleware: []process.Middleware{},
		hooks:      hooks.NewManager(),
		validator:  validate.NewManager(),
	}
}

// WithSource adds a source.
func (b *Builder) WithSource(s source.Source) *Builder {
	b.sources = append(b.sources, s)
	return b
}

// WithMiddleware adds source middleware.
func (b *Builder) WithMiddleware(m process.Middleware) *Builder {
	b.middleware = append(b.middleware, m)
	return b
}

// WithHook registers a lifecycle hook.
func (b *Builder) WithHook(h hooks.Hook) *Builder {
	b.hooks.Register(h)
	return b
}

// WithValidation registers a validation rule.
func (b *Builder) WithValidation(key string, r validate.Rule) *Builder {
	b.validator.Register(key, r)
	return b
}

// WithStructValidation registers validation rules from struct tags.
func (b *Builder) WithStructValidation(cfg any) *Builder {
	sv := validate.NewStructValidator(b.validator)
	_ = sv.RegisterStruct("", cfg)
	return b
}

// WithStructRule registers a struct-level validation rule.
func (b *Builder) WithStructRule(r validate.StructRule) *Builder {
	b.validator.RegisterStructRule(r)
	return b
}

// Validator returns the validation manager.
func (b *Builder) Validator() *validate.Manager {
	return b.validator
}

// Build assembles and loads the config.
func (b *Builder) Build() (*Config, error) {
	merged, err := b.loadSources()
	if err != nil {
		return nil, err
	}

	if err := b.runPreLoadHooks(merged); err != nil {
		return nil, err
	}

	b.cfg.Load(merged)

	if err := b.runPostLoadHooks(merged); err != nil {
		return nil, err
	}

	if err := b.runValidation(); err != nil {
		return nil, err
	}

	return b.cfg, nil
}

func (b *Builder) loadSources() (map[string]any, error) {
	merged := make(map[string]any)
	mw := process.Chain(b.middleware...)

	for _, src := range b.sources {
		if err := b.loadSource(src, mw, merged); err != nil {
			return nil, err
		}
	}

	return merged, nil
}

func (b *Builder) loadSource(src source.Source, mw process.Middleware, merged map[string]any) error {
	wrapped := mw(src)
	data, err := wrapped.Load()
	if err != nil {
		return fmt.Errorf("source %s: %w", src.Name(), err)
	}

	mergeData(merged, data)
	return nil
}

func mergeData(dst, src map[string]any) {
	for k, v := range src {
		dst[k] = v
	}
}

func (b *Builder) runPreLoadHooks(data map[string]any) error {
	return b.hooks.Run(hooks.PreLoad, data)
}

func (b *Builder) runPostLoadHooks(data map[string]any) error {
	return b.hooks.Run(hooks.PostLoad, data)
}

func (b *Builder) runValidation() error {
	return b.validator.Validate(b.cfg.All())
}
