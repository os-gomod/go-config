package config

import (
	"github.com/os-gomod/go-config/process"
	"github.com/os-gomod/go-config/source"
)

// WithEnv adds an environment source.
func (b *Builder) WithEnv(prefix string, priority int) *Builder {
	return b.WithSource(source.NewEnvSource(prefix, priority))
}

// WithFile adds a file source.
func (b *Builder) WithFile(path string, priority int) *Builder {
	return b.WithSource(source.NewFileSource(path, priority))
}

// WithMemory adds an in-memory source.
func (b *Builder) WithMemory(data map[string]any, priority int) *Builder {
	return b.WithSource(source.NewMemorySource(data, priority))
}

// WithEncryption enables encryption middleware.
func (b *Builder) WithEncryption(key string) *Builder {
	enc, _ := process.NewAESEncryptor(key)
	proc := process.NewEncryptionProcessor(enc, "ENC:")
	return b.withProcessor(proc)
}

// WithTemplates enables template processing.
func (b *Builder) WithTemplates(fn map[string]any) *Builder {
	proc := createTemplateProcessor(fn)
	return b.withProcessor(proc)
}

func createTemplateProcessor(fn map[string]any) *process.TemplateProcessor {
	tp := process.NewTemplateProcessor()
	for k, v := range fn {
		tp.AddFunc(k, v)
	}
	return tp
}

func (b *Builder) withProcessor(p process.Processor) *Builder {
	return b.WithMiddleware(func(s source.Source) source.Source {
		return process.NewProcessingSource(s, p)
	})
}

// // WithReload enables hot reload from file paths.
// func (b *Builder) WithReload(_ time.Duration) *Builder {
// 	// wiring handled externally via watch package
// 	return b
// }
