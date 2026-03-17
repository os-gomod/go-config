// Package config_test provides examples and tests for the configuration library.
package config_test

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/os-gomod/go-config"
	"github.com/os-gomod/go-config/types"
	"github.com/os-gomod/go-config/validate"
)

// AppConfig represents the complete application configuration.
type AppConfig struct {
	Server     ServerConfig     `config:"server"     validate:"required"`
	Database   DatabaseConfig   `config:"database"   validate:"required"`
	Logging    LoggingConfig    `config:"logging"    validate:"required"`
	Cache      CacheConfig      `config:"cache"`
	Security   SecurityConfig   `config:"security"   validate:"required"`
	Features   FeaturesConfig   `config:"features"`
	Monitoring MonitoringConfig `config:"monitoring"`
	Tracing    TracingConfig    `config:"tracing"`
}

// ServerConfig holds server-related configuration.
type ServerConfig struct {
	Host         string        `config:"host"          validate:"required,hostname|ip"`
	Port         int           `config:"port"          validate:"required,min=1,max=65535"`
	Mode         string        `config:"mode"          validate:"required,oneof=development staging production"`
	Timeout      time.Duration `config:"timeout"       validate:"min=1s"`
	ReadTimeout  time.Duration `config:"read_timeout"  validate:"min=1s"`
	WriteTimeout time.Duration `config:"write_timeout" validate:"min=1s"`
}

// DatabaseConfig holds database connection configuration.
type DatabaseConfig struct {
	Driver          string        `config:"driver"             validate:"required,oneof=postgres mysql sqlite"`
	Host            string        `config:"host"               validate:"required_if=Driver postgres mysql"`
	Port            int           `config:"port"               validate:"required_if=Driver postgres mysql,min=1,max=65535"`
	Name            string        `config:"name"               validate:"required"`
	User            string        `config:"user"               validate:"required"`
	Password        string        `config:"password"`
	MaxConns        int           `config:"max_conns"          validate:"min=1,max=100"`
	MinConns        int           `config:"min_conns"          validate:"min=1,ltefield=MaxConns"`
	ConnMaxLifetime time.Duration `config:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `config:"conn_max_idle_time"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level  string `config:"level"  validate:"required,oneof=debug info warn error"`
	Format string `config:"format" validate:"required,oneof=json text"`
	Output string `config:"output" validate:"required,oneof=stdout stderr file"`
	File   string `config:"file"   validate:"required_if=Output file"`
}

// CacheConfig holds cache configuration.
type CacheConfig struct {
	Enabled bool   `config:"enabled"`
	Driver  string `config:"driver"  validate:"required_if=Enabled true,oneof=redis memcached"`
	TTL     int    `config:"ttl"     validate:"min=0"`
	Redis   struct {
		Host     string `config:"host" validate:"required_if=Driver redis"`
		Port     int    `config:"port" validate:"required_if=Driver redis,min=1,max=65535"`
		Password string `config:"password"`
		DB       int    `config:"db" validate:"min=0,max=15"`
	} `config:"redis"`
}

// SecurityConfig holds security-related configuration.
type SecurityConfig struct {
	JWTSecret          string   `config:"jwt_secret"           validate:"required,min=32"`
	EncryptionKey      string   `config:"encryption_key"       validate:"required,min=32"`
	CORSEnabled        bool     `config:"cors_enabled"`
	CORSAllowedOrigins []string `config:"cors_allowed_origins" validate:"dive,url"`
	RateLimit          int      `config:"rate_limit"           validate:"min=0"`
}

// FeaturesConfig holds feature flag configuration.
type FeaturesConfig struct {
	EnableMetrics   bool `config:"enable_metrics"`
	EnableTracing   bool `config:"enable_tracing"`
	EnableProfiling bool `config:"enable_profiling"`
	BetaFeatures    bool `config:"beta_features"`
}

// MonitoringConfig holds monitoring configuration.
type MonitoringConfig struct {
	MetricsPort        int    `config:"metrics_port"         validate:"min=1,max=65535"`
	HealthCheckEnabled bool   `config:"health_check_enabled"`
	HealthCheckPath    string `config:"health_check_path"`
}

// TracingConfig holds tracing configuration.
type TracingConfig struct {
	Provider   string  `config:"provider"    validate:"oneof=jaeger zipkin none"`
	Endpoint   string  `config:"endpoint"    validate:"required_if=Provider jaeger zipkin,url"`
	SampleRate float64 `config:"sample_rate" validate:"min=0,max=1"`
}

// ExampleNew demonstrates creating a new configuration instance.
func ExampleNew() {
	// Make example deterministic regardless of local config/environment.
	prevHost, hadHost := os.LookupEnv("APP_SERVER_HOST")
	prevPort, hadPort := os.LookupEnv("APP_SERVER_PORT")
	if err := os.Setenv("APP_SERVER_HOST", "localhost"); err != nil {
		panic(err)
	}
	if err := os.Setenv("APP_SERVER_PORT", "8080"); err != nil {
		panic(err)
	}
	defer func() {
		if hadHost {
			_ = os.Setenv("APP_SERVER_HOST", prevHost)
		} else {
			_ = os.Unsetenv("APP_SERVER_HOST")
		}

		if hadPort {
			_ = os.Setenv("APP_SERVER_PORT", prevPort)
		} else {
			_ = os.Unsetenv("APP_SERVER_PORT")
		}
	}()

	// Create configuration with file and environment sources
	cfg, err := config.New(
		config.WithFile("config.yaml"),
		config.WithEnv("APP_"),
	)
	if err != nil {
		panic(err)
	}
	defer cfg.Close(context.Background())

	// Get values
	port := cfg.GetInt("server.port")
	host := cfg.GetString("server.host")

	fmt.Printf("Server running on %s:%d\n", host, port)

	// Output:
	// Server running on localhost:8080
}

// ExampleNew_withObserver demonstrates observing configuration changes.
func ExampleNew_withObserver() {
	cfg, err := config.New(
		config.WithFile("config.yaml"),
		config.WithObserver(func(_ context.Context, event types.Event) {
			fmt.Printf("Config changed: %s %s\n", event.Key, event.Type)
		}),
	)
	if err != nil {
		panic(err)
	}
	defer cfg.Close(context.Background())

	// Watch for changes
	if setErr := cfg.Watch(context.Background()); setErr != nil {
		panic(fmt.Errorf("Failed to start hot reload watcher: %w", setErr))
	}

	// Output:
}

// ExampleConfig_Bind demonstrates struct binding.
func ExampleConfig_Bind() {
	prevHost, hadHost := os.LookupEnv("APP_SERVER_HOST")
	prevPort, hadPort := os.LookupEnv("APP_SERVER_PORT")
	prevDBUser, hadDBUser := os.LookupEnv("APP_DATABASE_USER")
	prevDBHost, hadDBHost := os.LookupEnv("APP_DATABASE_HOST")
	prevDBPort, hadDBPort := os.LookupEnv("APP_DATABASE_PORT")
	prevDBName, hadDBName := os.LookupEnv("APP_DATABASE_NAME")

	if err := os.Setenv("APP_SERVER_HOST", "localhost"); err != nil {
		panic(err)
	}
	if err := os.Setenv("APP_SERVER_PORT", "8080"); err != nil {
		panic(err)
	}
	if err := os.Setenv("APP_DATABASE_USER", "admin"); err != nil {
		panic(err)
	}
	if err := os.Setenv("APP_DATABASE_HOST", "db.local"); err != nil {
		panic(err)
	}
	if err := os.Setenv("APP_DATABASE_PORT", "5432"); err != nil {
		panic(err)
	}
	if err := os.Setenv("APP_DATABASE_NAME", "mydb"); err != nil {
		panic(err)
	}
	defer func() {
		if hadHost {
			_ = os.Setenv("APP_SERVER_HOST", prevHost)
		} else {
			_ = os.Unsetenv("APP_SERVER_HOST")
		}
		if hadPort {
			_ = os.Setenv("APP_SERVER_PORT", prevPort)
		} else {
			_ = os.Unsetenv("APP_SERVER_PORT")
		}
		if hadDBUser {
			_ = os.Setenv("APP_DATABASE_USER", prevDBUser)
		} else {
			_ = os.Unsetenv("APP_DATABASE_USER")
		}
		if hadDBHost {
			_ = os.Setenv("APP_DATABASE_HOST", prevDBHost)
		} else {
			_ = os.Unsetenv("APP_DATABASE_HOST")
		}
		if hadDBPort {
			_ = os.Setenv("APP_DATABASE_PORT", prevDBPort)
		} else {
			_ = os.Unsetenv("APP_DATABASE_PORT")
		}
		if hadDBName {
			_ = os.Setenv("APP_DATABASE_NAME", prevDBName)
		} else {
			_ = os.Unsetenv("APP_DATABASE_NAME")
		}
	}()

	cfg, err := config.New(
		config.WithFile("config.yaml"),
		config.WithEnv("APP_"),
	)
	if err != nil {
		panic(err)
	}
	defer cfg.Close(context.Background())

	var appCfg AppConfig
	if setErr := cfg.Bind(&appCfg); setErr != nil {
		panic(fmt.Errorf("Failed to bind config: %w", setErr))
	}

	fmt.Printf("Server: %s:%d\n", appCfg.Server.Host, appCfg.Server.Port)
	fmt.Printf("Database: %s@%s:%d/%s\n",
		appCfg.Database.User,
		appCfg.Database.Host,
		appCfg.Database.Port,
		appCfg.Database.Name,
	)

	// Output:
	// Server: localhost:8080
	// Database: admin@db.local:5432/mydb
}

// ExampleConfig_Validate demonstrates configuration validation.
func ExampleConfig_Validate() {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"server.host":   "localhost",
			"server.port":   8080,
			"logging.level": "info",
		}),
	)
	if err != nil {
		panic(err)
	}
	defer cfg.Close(context.Background())

	// Build validation plan
	plan := validate.NewBuilder().
		Required("server.host", "server host is required").
		Range("server.port", 1, 65535, "server port must be between 1 and 65535").
		Enum("logging.level", []string{"debug", "info", "warn", "error"}, "invalid log level").
		Build()

	// Validate
	if setErr := cfg.Validate(plan); setErr != nil {
		panic(fmt.Errorf("Validation failed: %w", setErr))
	}

	fmt.Println("Configuration is valid")

	// Output:
	// Configuration is valid
}

// ExampleConfig_Snapshot demonstrates configuration snapshots.
func ExampleConfig_Snapshot() {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"server.port": 8080,
		}),
	)
	if err != nil {
		panic(err)
	}
	defer cfg.Close(context.Background())

	// Take a snapshot
	snap := cfg.Snapshot()
	fmt.Printf("Snapshot ID: %d\n", snap.ID())

	// Modify configuration
	cfg.Set("server.port", 8081)

	// Later, restore from snapshot
	if setErr := cfg.Restore(snap); setErr != nil {
		panic(fmt.Errorf("Failed to restore: %w", setErr))
	}

	fmt.Println("Configuration restored to previous state")

	// Output:
	// Snapshot ID: 1
	// Configuration restored to previous state
}

// ExampleConfig_Export demonstrates exporting configuration.
func ExampleConfig_Export() {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"server.host": "localhost",
			"server.port": 8080,
		}),
	)
	if err != nil {
		panic(err)
	}
	defer cfg.Close(context.Background())

	// Export to JSON
	jsonData, err := cfg.ExportJSON()
	if err != nil {
		panic(err)
	}
	fmt.Println("JSON empty:", len(jsonData) == 0)

	// Export to YAML
	yamlData, err := cfg.ExportYAML()
	if err != nil {
		panic(err)
	}
	fmt.Println("YAML empty:", len(yamlData) == 0)

	// Output:
	// JSON empty: false
	// YAML empty: false
}

// ExampleConfig_encryption demonstrates encrypted configuration values.
func ExampleConfig_encryption() {
	// Initialize with encryption key
	key := []byte("32-byte-encryption-key-here!!") // In production, use secure key

	cfg, err := config.New(
		config.WithMemory(map[string]any{}),
		config.WithEncryption(key),
	)
	if err != nil {
		panic(err)
	}
	defer cfg.Close(context.Background())

	// Encrypt a value
	encrypted, err := cfg.Encrypt("my-secret-password")
	if err != nil {
		panic(err)
	}
	fmt.Println("Encrypted:", encrypted.Ciphertext != "")

	// Decrypt a value
	decrypted, err := cfg.Decrypt(encrypted)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Decrypted: %s\n", decrypted)

	// Output:
	// Encrypted: true
	// Decrypted: my-secret-password
}

// ExampleLoadEnv demonstrates loading from environment variables.
func ExampleLoadEnv() {
	// Set some environment variables
	prevPort, hadPort := os.LookupEnv("APP_SERVER_PORT")
	prevHost, hadHost := os.LookupEnv("APP_SERVER_HOST")
	prevDBName, hadDBName := os.LookupEnv("APP_DATABASE_NAME")
	if err := os.Setenv("APP_SERVER_PORT", "8080"); err != nil {
		panic(err)
	}
	if err := os.Setenv("APP_SERVER_HOST", "localhost"); err != nil {
		panic(err)
	}
	if err := os.Setenv("APP_DATABASE_NAME", "mydb"); err != nil {
		panic(err)
	}
	defer func() {
		if hadPort {
			_ = os.Setenv("APP_SERVER_PORT", prevPort)
		} else {
			_ = os.Unsetenv("APP_SERVER_PORT")
		}
		if hadHost {
			_ = os.Setenv("APP_SERVER_HOST", prevHost)
		} else {
			_ = os.Unsetenv("APP_SERVER_HOST")
		}
		if hadDBName {
			_ = os.Setenv("APP_DATABASE_NAME", prevDBName)
		} else {
			_ = os.Unsetenv("APP_DATABASE_NAME")
		}
	}()

	cfg, err := config.LoadEnv("APP_")
	if err != nil {
		panic(err)
	}
	defer cfg.Close(context.Background())

	fmt.Println("Server Port:", cfg.GetInt("server.port"))
	fmt.Println("Server Host:", cfg.GetString("server.host"))
	fmt.Println("Database Name:", cfg.GetString("database.name"))

	// Output:
	// Server Port: 8080
	// Server Host: localhost
	// Database Name: mydb
}

// ExampleConfig_hotReload demonstrates hot reload functionality.
func ExampleConfig_hotReload() {
	cfg, err := config.New(
		config.WithFile("config.yaml"),
	)
	if err != nil {
		panic(err)
	}
	defer cfg.Close(context.Background())

	// Start watching for changes
	if setErr := cfg.Watch(context.Background()); setErr != nil {
		panic(fmt.Errorf("Failed to start hot reload watcher: %w", setErr))
	}

	fmt.Println("Hot reload watcher started")

	// Output:
	// Hot reload watcher started
}

// ExampleConfig_Merge demonstrates merging configurations.
func ExampleConfig_Merge() {
	// Create base configuration
	base, err := config.New(
		config.WithMemory(map[string]any{
			"server.host": "localhost",
			"server.port": 8080,
		}),
	)
	if err != nil {
		panic(err)
	}
	defer base.Close(context.Background())

	// Create override configuration
	override, err := config.New(
		config.WithMemory(map[string]any{
			"server.port": 9090, // Override port
			"debug":       true, // Add new key
		}),
	)
	if err != nil {
		panic(err)
	}
	defer override.Close(context.Background())

	// Merge configurations
	if setErr := base.Merge(override); setErr != nil {
		panic(fmt.Errorf("Failed to merge configurations: %w", setErr))
	}

	fmt.Println("Host:", base.GetString("server.host")) // localhost (unchanged)
	fmt.Println("Port:", base.GetInt("server.port"))    // 9090 (overridden)
	fmt.Println("Debug:", base.GetBool("debug"))        // true (added)

	// Output:
	// Host: localhost
	// Port: 9090
	// Debug: true
}

// ExampleConfig_templateProcessing demonstrates template processing.
func ExampleConfig_templateProcessing() {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"app.name":   "myapp",
			"app.home":   "/opt/${app.name}",
			"app.log":    "${app.home}/logs",
			"app.config": "${app.home}/config",
		}),
	)
	if err != nil {
		panic(err)
	}
	defer cfg.Close(context.Background())

	// Process templates
	if setErr := cfg.ProcessTemplates(); setErr != nil {
		panic(fmt.Errorf("Failed to process templates: %w", setErr))
	}

	fmt.Println("Home:", cfg.GetString("app.home"))     // /opt/myapp
	fmt.Println("Log:", cfg.GetString("app.log"))       // /opt/myapp/logs
	fmt.Println("Config:", cfg.GetString("app.config")) // /opt/myapp/config

	// Output:
	// Home: /opt/myapp
	// Log: /opt/myapp/logs
	// Config: /opt/myapp/config
}

// ExampleConfig_context demonstrates context-aware operations.
func ExampleConfig_context() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"server.port": 8080,
		}),
	)
	if err != nil {
		panic(err)
	}
	defer cfg.Close(ctx)

	// Use context for timeout-aware loading
	if setErr := cfg.Reload(ctx); setErr != nil {
		panic(fmt.Errorf("Failed to reload config: %w", setErr))
	}

	// Attach config to context for propagation
	ctx = config.ContextWithConfig(ctx, cfg)

	// Retrieve config from context
	if c := config.ConfigFromContext(ctx); c != nil {
		fmt.Println("Port:", c.GetInt("server.port"))
	}

	// Output:
	// Port: 8080
}

// ExampleGlobal demonstrates using the global configuration instance.
func ExampleGlobal() {
	prevPort, hadPort := os.LookupEnv("APP_SERVER_PORT")
	if err := os.Setenv("APP_SERVER_PORT", "8080"); err != nil {
		panic(err)
	}
	defer func() {
		if hadPort {
			_ = os.Setenv("APP_SERVER_PORT", prevPort)
		} else {
			_ = os.Unsetenv("APP_SERVER_PORT")
		}
	}()

	// Initialize global configuration
	if err := config.Init(
		config.WithFile("config.yaml"),
		config.WithEnv("APP_"),
	); err != nil {
		panic(err)
	}
	defer config.CloseGlobal(context.Background())

	// Use global instance anywhere in your application
	cfg := config.Global()
	if cfg != nil {
		fmt.Println("Port:", cfg.GetInt("server.port"))
	}

	// Output:
	// Port: 8080
}
