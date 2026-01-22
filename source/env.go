package source

import (
	"os"
	"strings"
)

// EnvSource loads from environment variables.
type EnvSource struct {
	BaseSource
	prefix string
}

func NewEnvSource(prefix string, priority int) *EnvSource {
	return &EnvSource{
		BaseSource: NewBase("env:"+prefix, priority),
		prefix:     prefix,
	}
}

func (s *EnvSource) Load() (map[string]any, error) {
	out := make(map[string]any)
	for _, e := range os.Environ() {
		if key, value := parseEnvVar(e, s.prefix); key != "" {
			out[key] = value
		}
	}
	return out, nil
}

func parseEnvVar(envVar, prefix string) (key string, value string) {
	kv := strings.SplitN(envVar, "=", 2)
	if len(kv) != 2 || !strings.HasPrefix(kv[0], prefix) {
		return "", ""
	}

	key = strings.ToLower(
		strings.ReplaceAll(
			strings.TrimPrefix(kv[0], prefix),
			"_",
			".",
		),
	)
	value = kv[1]
	return key, value
}
