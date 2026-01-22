package source

// EnvOverrideSource applies environment-specific overrides.
type EnvOverrideSource struct {
	BaseSource
	env       string
	overrides map[string]map[string]any
}

func NewEnvOverride(env string, overrides map[string]map[string]any, priority int) *EnvOverrideSource {
	return &EnvOverrideSource{
		BaseSource: NewBase("env-override:"+env, priority),
		env:        env,
		overrides:  overrides,
	}
}

func (s *EnvOverrideSource) Load() (map[string]any, error) {
	if data, ok := s.overrides[s.env]; ok {
		return copySourceData(data), nil
	}
	return map[string]any{}, nil
}
