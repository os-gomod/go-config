# Go-Config: Comprehensive Configuration Management for Go

A production-ready, feature-rich configuration management library for Go applications that provides a unified, type-safe way to handle configuration from multiple sources with validation, encryption, and hot-reload capabilities.

## Features

### 🚀 **Core Features**
- **Multi-source configuration**: Load from files, environment variables, memory, and custom sources
- **Priority-based merging**: Control source precedence with configurable priorities
- **Type-safe access**: Get typed values with automatic conversion
- **Struct binding**: Bind configuration to Go structs with tags
- **Validation**: Built-in and custom validation rules with struct tag support
- **Encryption**: AES-GCM encryption for sensitive values
- **Template processing**: Template strings with custom functions
- **Lifecycle hooks**: Custom logic at configuration lifecycle stages
- **Change observers**: React to configuration changes in real-time
- **Snapshots**: Version and restore configuration states
- **Hot reload**: Watch files and reload configuration automatically
- **Export**: Export to JSON, YAML, and generate JSON Schema

### 🛡️ **Production Ready**
- **Thread-safe**: All operations are safe for concurrent use
- **Atomic updates**: Batch updates with change tracking
- **Freeze/Unfreeze**: Immutable configuration when needed
- **Error handling**: Comprehensive error reporting and validation
- **Clean API**: Fluent builder pattern and intuitive interfaces

## Installation

```bash
go get github.com/os-gomod/go-config
```

## Quick Start

### Basic Usage

```go
package main

import (
    "fmt"
    "github.com/os-gomod/go-config"
)

func main() {
    cfg, err := config.NewBuilder().
        WithMemory(map[string]any{
            "app.name": "MyApp",
            "app.port": 8080,
        }, 10).
        Build()
    
    if err != nil {
        panic(err)
    }
    
    port := config.GetTyped(cfg, "app.port", func(v any) (int, bool) {
        if i, ok := v.(int); ok {
            return i, true
        }
        return 0, false
    }).OrDefault(3000)
    
    fmt.Printf("Server running on port %d\n", port)
}
```

### Struct Binding Example

```go
type AppConfig struct {
    Server struct {
        Host string `config:"host" validate:"required"`
        Port int    `config:"port" validate:"min=1,max=65535"`
    } `config:"server"`
    Database struct {
        URL string `config:"url" validate:"required"`
    } `config:"database"`
}

func main() {
    cfg, err := config.NewBuilder().
        WithFile("config.yaml", 20).
        WithEnv("APP_", 30).
        WithStructValidation(&AppConfig{}).
        Build()
    
    var appCfg AppConfig
    if err := config.Bind(cfg.All(), &appCfg); err != nil {
        panic(err)
    }
    
    fmt.Printf("Server: %s:%d\n", appCfg.Server.Host, appCfg.Server.Port)
}
```

## Configuration Sources

### File Source (YAML/JSON)
```go
config.NewBuilder().
    WithFile("config.yaml", 10)  // Priority 10
```

### Environment Variables
```go
config.NewBuilder().
    WithEnv("APP_", 20)  // Prefix "APP_", priority 20
```

### Memory Source
```go
config.NewBuilder().
    WithMemory(map[string]any{
        "server.host": "localhost",
        "server.port": 8080,
    }, 5)
```

### Composite Source
```go
composite := source.NewComposite(
    "app-config",
    100,
    source.NewMemorySource(defaults, 1),
    source.NewEnvSource("APP_", 2),
)
```

## Validation

### Struct Tag Validation
```go
type ServerConfig struct {
    Host string `config:"host" validate:"required"`
    Port int    `config:"port" validate:"required,min=1,max=65535"`
    Mode string `config:"mode" validate:"required,oneof=dev staging prod"`
}
```

### Custom Validation Rules
```go
type DatabaseConnectionRule struct{}

func (DatabaseConnectionRule) ValidateStruct(s any) error {
    cfg := s.(*AppConfig)
    if cfg.Database.Driver == "sqlite" && cfg.Database.Host != "localhost" {
        return fmt.Errorf("sqlite must use localhost")
    }
    return nil
}

config.NewBuilder().
    WithStructRule(DatabaseConnectionRule{})
```

## Encryption

```go
config.NewBuilder().
    WithEncryption("my-32-byte-encryption-key-here")
```

Encrypted values are marked with `ENC()` prefix:
```yaml
database:
  password: "ENC(RZq9xKZc2Zx8xkHcQq9m8FqP9QXcPZcJ1yM=)"
```

## Lifecycle Hooks

```go
type MetricsHook struct {
    loadCount int
}

func (m *MetricsHook) OnPostLoad(data map[string]any) error {
    m.loadCount++
    log.Printf("Config loaded %d times", m.loadCount)
    return nil
}

config.NewBuilder().
    WithHook(&MetricsHook{})
```

## Observers and Hot Reload

```go
// React to changes
cfg.ObserveFunc(func(changes []core.Change) {
    for _, c := range changes {
        fmt.Printf("%s changed: %v → %v\n", c.Key, c.Old, c.New)
    }
})

// Watch for file changes
watcher := watch.New(5*time.Second, []string{"config.yaml"})
watcher.Start(func() {
    fmt.Println("Config changed, reloading...")
})
```

## Advanced Features

### Template Processing
```go
config.NewBuilder().
    WithTemplates(map[string]any{
        "upper": strings.ToUpper,
        "env": func() string { return os.Getenv("ENV") },
    })
```

### Snapshots
```go
snapMgr := snapshot.NewManager(5)  // Keep last 5 snapshots
snap1 := snapMgr.Take(cfg.All())

// Restore
if restored, ok := snapMgr.Get(1); ok {
    cfg.Load(restored.Data)
}
```

### Export
```go
// Export to JSON
jsonExp := export.JSONExporter{}
jsonData, _ := jsonExp.Export(cfg.All())

// Generate JSON Schema
schema, _ := export.GenerateSchema(&AppConfig{})
```

## Presets

```go
// Development preset
devCfg, _ := config.DevelopmentPreset().
    WithMemory(defaults, 10).
    Build()

// Production preset
prodCfg, _ := config.ProductionPreset().
    WithMemory(defaults, 10).
    Build()
```

## Best Practices

1. **Use struct validation**: Always validate configuration with struct tags
2. **Encrypt sensitive data**: Use encryption middleware for secrets
3. **Set proper priorities**: Lower priority numbers = higher precedence
4. **Use typed accessors**: Avoid type assertions with `core.GetTyped`
5. **Implement observers**: React to configuration changes appropriately
6. **Create snapshots**: Version your configuration in production
7. **Use hooks for metrics**: Track configuration lifecycle events
8. **Freeze in production**: Prevent accidental changes in production

## Example Configuration

See the comprehensive example in `main.go` that demonstrates all features including:
- Multiple configuration sources
- Struct validation and binding
- Encryption and template processing
- Observers and hot reload
- Snapshots and versioning
- Export and schema generation

## API Reference

### Core Packages
- `config`: Builder pattern for configuration assembly
- `core`: Thread-safe configuration container
- `bind`: Struct binding utilities
- `source`: Configuration source implementations
- `process`: Middleware and processors
- `validate`: Validation framework
- `hooks`: Lifecycle hooks
- `export`: Export utilities
- `snapshot`: Configuration snapshots
- `watch`: File watching and hot reload

## Contributing

Contributions are welcome! Please read our contributing guidelines before submitting pull requests.

## License

MIT License - see LICENSE file for details.

## Support

- Issues: [GitHub Issues](https://github.com/os-gomod/go-config/issues)
- Documentation: [GoDoc](https://pkg.go.dev/github.com/os-gomod/go-config)

---

**Go-Config** is maintained by the OS-Gomod team. Built with ❤️ for the Go community.