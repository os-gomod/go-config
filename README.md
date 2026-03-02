# Go Configuration Library

A production-grade, zero-duplication, zero-memory-leak, high-performance configuration management library for Go.

## Features

### Core Features
- **Multi-source Configuration**: File (YAML, JSON, TOML), Environment, Memory, Remote sources
- **Priority-based Merging**: Configurable merge strategies with source priority
- **Immutable + Mutable Modes**: Support for both immutable snapshots and mutable updates
- **Atomic Updates**: Lock-free reads with atomic copy-on-write updates
- **Struct Binding**: Type-safe binding with cached reflection metadata
- **Type-safe Getters**: Strongly typed value access methods
- **Validation**: Built-in validators with struct tag support
- **Encryption**: AES-GCM encryption for sensitive values
- **Template Processing**: Variable interpolation in configuration values
- **Observers**: Non-blocking event notification system
- **Lifecycle Hooks**: Before/after hooks for load, set, delete operations
- **Snapshots**: Time-travel queries and configuration recovery
- **Hot Reload**: File watching with automatic reload
- **Export**: JSON, YAML, TOML, Env format export
- **Schema Generation**: Automatic schema generation from structs

### Performance Features
- **O(1) Lock-free Reads**: Zero-allocation reads using atomic pointers
- **Copy-on-Write Updates**: Atomic state swaps without locks
- **sync.Pool**: Reusable objects for minimal allocations
- **Cached Reflection**: Struct metadata cached after first use

### Safety Features
- **Zero Memory Leaks**: Proper cleanup, no goroutine leaks
- **Zero Race Conditions**: Thread-safe by design
- **Zero Unsafe Pointers**: No unsafe package usage
- **Context Support**: Graceful shutdown with context

## Installation

```bash
go get github.com/os-gomod/go-config
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/os-gomod/go-config/config"
)

type AppConfig struct {
    Server   ServerConfig   `config:"server"`
    Database DatabaseConfig `config:"database"`
}

type ServerConfig struct {
    Host         string        `config:"host"`
    Port         int           `config:"port"`
    ReadTimeout  time.Duration `config:"read_timeout"`
    WriteTimeout time.Duration `config:"write_timeout"`
}

type DatabaseConfig struct {
    Host     string `config:"host"`
    Port     int    `config:"port"`
    Name     string `config:"name"`
    User     string `config:"user"`
    Password string `config:"password"`
}

func main() {
    // Create configuration with multiple sources
    cfg, err := config.New(
        config.WithFile("config.yaml"),
        config.WithEnv("APP_"),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer cfg.Close(context.Background())

    // Type-safe value access
    port := cfg.GetInt("server.port")
    host := cfg.GetString("server.host")
    timeout := cfg.GetDuration("server.read_timeout")

    fmt.Printf("Server running on %s:%d\n", host, port)

    // Struct binding
    var appCfg AppConfig
    if err := cfg.Bind(&appCfg); err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Database: %s@%s:%d/%s\n",
        appCfg.Database.User,
        appCfg.Database.Host,
        appCfg.Database.Port,
        appCfg.Database.Name,
    )
}
```

## Configuration Sources

### File Source

```go
// YAML file
cfg, _ := config.New(config.WithFile("config.yaml"))

// JSON file
cfg, _ := config.New(config.WithFile("config.json"))

// TOML file
cfg, _ := config.New(config.WithFile("config.toml"))
```

### Environment Source

```go
// With prefix
cfg, _ := config.New(config.WithEnv("APP_"))

// APP_SERVER_PORT=8080 becomes server.port
// APP_DATABASE_NAME=mydb becomes database.name
```

### Memory Source

```go
cfg, _ := config.New(
    config.WithMemory(map[string]any{
        "server.host": "localhost",
        "server.port": 8080,
    }),
)
```

### Remote Source

```go
cfg, _ := config.New(
    config.WithRemote("https://config.example.com/api/config",
        source.WithRemoteTTL(5*time.Minute),
    ),
)
```

### Multiple Sources

```go
// Sources are merged with priority: memory > env > file > remote
cfg, _ := config.New(
    config.WithFile("config.yaml"),        // Priority 10
    config.WithEnv("APP_"),                // Priority 20
    config.WithMemory(map[string]any{}),   // Priority 30 (highest)
)
```

## Value Access

```go
// Basic getters
host := cfg.GetString("server.host")
port := cfg.GetInt("server.port")
enabled := cfg.GetBool("feature.enabled")
timeout := cfg.GetDuration("server.timeout")

// With defaults
port := cfg.GetIntDefault("server.port", 8080)
host := cfg.GetStringDefault("server.host", "localhost")

// Check existence
if cfg.Has("server.port") {
    // ...
}

// Raw value access
value, exists := cfg.Get("server.port")
if exists {
    fmt.Println(value.String())
    fmt.Println(value.Int())
    fmt.Println(value.Source()) // file, env, memory, etc.
}
```

## Struct Binding

```go
type ServerConfig struct {
    Host    string `config:"host" validate:"required"`
    Port    int    `config:"port" validate:"required,min=1,max=65535"`
    Timeout time.Duration `config:"timeout"`
}

cfg, _ := config.New(config.WithFile("config.yaml"))

var server ServerConfig
if err := cfg.Bind(&server); err != nil {
    log.Fatal(err)
}
```

## Validation

```go
// Using validation plan
plan := validate.NewBuilder().
    Required("server.host", "server host is required").
    Range("server.port", 1, 65535, "port must be valid").
    Pattern("server.name", "^[a-z]+$", "name must be lowercase").
    Enum("log.level", []string{"debug", "info", "warn", "error"}, "invalid level").
    Build()

if err := cfg.Validate(plan); err != nil {
    log.Fatal(err)
}

// Using struct tags
type Config struct {
    Host  string `validate:"required"`
    Port  int    `validate:"required,min=1,max=65535"`
    Email string `validate:"email"`
    URL   string `validate:"url"`
}
```

## Observers

```go
// Register observer
cancel, _ := cfg.Observe(func(ctx context.Context, event config.Event) error {
    fmt.Printf("Config changed: %s %s\n", event.Key, event.Type)
    return nil
})
defer cancel()

// Or use WithObserver option
cfg, _ := config.New(
    config.WithFile("config.yaml"),
    config.WithObserver(func(ctx context.Context, event config.Event) error {
        // Handle change
        return nil
    }),
)
```

## Hot Reload

```go
cfg, _ := config.New(
    config.WithFile("config.yaml"),
    config.WithObserver(func(ctx context.Context, event config.Event) {
        if event.Type == config.EventReload {
            fmt.Println("Configuration reloaded!")
            // Re-bind struct
            var newCfg AppConfig
            cfg.Bind(&newCfg)
        }
    }),
)

// Start watching
if err := cfg.Watch(context.Background()); err != nil {
    log.Fatal(err)
}
```

## Snapshots

```go
// Take snapshot
snap := cfg.Snapshot()
fmt.Printf("Snapshot ID: %d\n", snap.ID())

// Modify configuration
cfg.Set("server.port", 9090)

// Restore from snapshot
cfg.Restore(snap)

// Get snapshot at a specific time
oldSnap := cfg.Snapshots().At(time.Now().Add(-time.Hour))
```

## Encryption

```go
// Initialize with encryption key (32 bytes for AES-256)
key := []byte("32-byte-encryption-key-here!!")
cfg, _ := config.New(
    config.WithFile("config.yaml"),
    config.WithEncryption(key),
)

// Encrypt value
encrypted, _ := cfg.Encrypt("my-secret-password")
fmt.Printf("Encrypted: %s\n", encrypted.Ciphertext)

// Decrypt value
decrypted, _ := cfg.Decrypt(encrypted)
fmt.Printf("Decrypted: %s\n", decrypted)
```

## Export

```go
// Export to JSON
jsonData, _ := cfg.ExportJSON()
fmt.Println(string(jsonData))

// Export to YAML
yamlData, _ := cfg.ExportYAML()
fmt.Println(string(yamlData))

// Export to TOML
tomlData, _ := cfg.ExportTOML()
fmt.Println(string(tomlData))

// Export to environment format
envData, _ := cfg.ExportEnv()
fmt.Println(string(envData))
```

## Context Support

```go
// Timeout-aware operations
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

if err := cfg.Reload(ctx); err != nil {
    log.Fatal(err)
}

// Store config in context
ctx = config.ContextWithConfig(ctx, cfg)

// Retrieve config from context
if c := config.ConfigFromContext(ctx); c != nil {
    port := c.GetInt("server.port")
}
```

## Architecture

### Memory Model

The library uses an immutable state model with atomic pointer swaps:

```
┌─────────────────────────────────────────────────────────────┐
│                        Config                                │
│  ┌─────────────────────────────────────────────────────┐    │
│  │                 atomic.Pointer[State]                │    │
│  └─────────────────────────────────────────────────────┘    │
│                           │                                  │
│                           ▼                                  │
│  ┌─────────────────────────────────────────────────────┐    │
│  │                     State                            │    │
│  │  ┌─────────────────────────────────────────────┐    │    │
│  │  │         map[string]Value (immutable)         │    │    │
│  │  └─────────────────────────────────────────────┘    │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

### Read Path (Lock-Free)

```
Get(key) → atomic.Load() → State → map[key] → Value
           O(1)           O(1)     O(1)
```

### Write Path (Copy-on-Write)

```
Set(key, value) → Lock → Copy State → Modify Copy → atomic.Store
```

### Package Structure

```
config/              # Main public API
├── core/            # State engine, observer manager
├── types/           # Core types (Value, Event, Error)
├── source/          # Configuration sources (file, env, memory, remote)
├── parser/          # File format parsers (YAML, JSON, TOML)
├── merge/           # Priority-based merging
├── validate/        # Validation with compiled plans
├── crypto/          # AES-GCM encryption
├── watch/           # Hot reload with file watching
├── snapshot/        # Immutable snapshots
├── export/          # Format export (JSON, YAML, TOML, Env)
├── bind/            # Struct binding with cached metadata
├── loader/          # Loading utilities with sync.Pool
└── internal/
    ├── pool/        # Optimized sync.Pool implementations
    ├── syncutil/    # Synchronization utilities
    └── reflectutil/ # Cached reflection utilities
```

## Performance

### Benchmarks

| Operation | Time | Allocations |
|-----------|------|-------------|
| Get (lock-free) | < 50ns | 0 |
| Set (copy-on-write) | < 500µs | minimal |
| Bind (cached) | < 1µs | minimal |
| Snapshot | < 10µs | proportional to size |

### Memory Safety

- No goroutine leaks (all goroutines have lifecycle control via context)
- No unbounded channels (all channels have fixed capacity)
- No circular references
- Proper cleanup methods on all resources

## Best Practices

### 1. Use Struct Binding for Type Safety

```go
type AppConfig struct {
    Server ServerConfig `config:"server"`
}

var cfg AppConfig
config.Bind(&cfg)
```

### 2. Use Context for Graceful Shutdown

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// On shutdown signal
go func() {
    <-shutdownSignal
    cancel()
}()

cfg.Close(ctx)
```

### 3. Use Observers for Reactivity

```go
cfg.Observe(func(ctx context.Context, event config.Event) error {
    // Rebuild dependent objects
    return nil
})
```

### 4. Take Snapshots Before Critical Changes

```go
snap := cfg.Snapshot()
// ... make changes ...
if somethingWentWrong {
    cfg.Restore(snap)
}
```

## Error Handling

The library uses structured errors:

```go
value, err := cfg.Get("key")
if err != nil {
    if config.IsNotFound(err) {
        // Handle missing key
    }
    if config.IsTypeMismatch(err) {
        // Handle type error
    }
}
```

## Testing

```bash
# Run tests
go test ./...

# Run benchmarks
go test -bench=. ./...

# Run with coverage
go test -cover ./...
```

## License

MIT License

---

**Go-Config** is maintained by the OS-Gomod team. Built with ❤️ for the Go community.