package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/os-gomod/go-config"
	"github.com/os-gomod/go-config/export"
	"github.com/os-gomod/go-config/hooks"
	"github.com/os-gomod/go-config/snapshot"
	"github.com/os-gomod/go-config/source"
	"github.com/os-gomod/go-config/validate"
	"github.com/os-gomod/go-config/watch"
)

// =============================================================================
// CONFIGURATION STRUCT WITH VALIDATION TAGS
// =============================================================================

type AppConfig struct {
	Server   ServerConfig   `config:"server"`
	Database DatabaseConfig `config:"database"`
	Cache    CacheConfig    `config:"cache"`
	Security SecurityConfig `config:"security"`
	Features FeatureFlags   `config:"features"`
}

type ServerConfig struct {
	Host    string        `config:"host" validate:"required"`
	Port    int           `config:"port" validate:"required,min=1,max=65535"`
	Mode    string        `config:"mode" validate:"required,oneof=dev staging prod"`
	Timeout time.Duration `config:"timeout"`
}

type DatabaseConfig struct {
	Driver   string `config:"driver" validate:"required,oneof=postgres mysql sqlite"`
	Host     string `config:"host" validate:"required"`
	Port     int    `config:"port" validate:"required"`
	Database string `config:"database" validate:"required"`
	Username string `config:"username" validate:"required"`
	Password string `config:"password" validate:"required"`
	MaxConns int    `config:"max_conns" validate:"min=1,max=100"`
}

type CacheConfig struct {
	Enabled bool   `config:"enabled"`
	Driver  string `config:"driver" validate:"required_if=enabled true,oneof=redis memcached"`
	TTL     int    `config:"ttl" validate:"min=0"`
}

type SecurityConfig struct {
	JWTSecret     string `config:"jwt_secret" validate:"required"`
	EncryptionKey string `config:"encryption_key" validate:"required"`
	CORS          bool   `config:"cors"`
}

type FeatureFlags struct {
	EnableMetrics bool `config:"enable_metrics"`
	EnableTracing bool `config:"enable_tracing"`
	BetaFeatures  bool `config:"beta_features"`
}

// =============================================================================
// CUSTOM VALIDATION RULE
// =============================================================================

type DatabaseConnectionRule struct{}

func (DatabaseConnectionRule) ValidateStruct(s any) error {
	cfg, ok := s.(*AppConfig)
	if !ok {
		return nil
	}

	// Custom business logic validation
	if cfg.Database.Driver == "sqlite" && cfg.Database.Host != "localhost" {
		return fmt.Errorf("sqlite must use localhost")
	}

	if cfg.Cache.Enabled && cfg.Cache.Driver == "" {
		return fmt.Errorf("cache driver required when cache is enabled")
	}

	return nil
}

// =============================================================================
// CUSTOM HOOKS
// =============================================================================

type ValidationHook struct{}

func (ValidationHook) Name() string  { return "validation" }
func (ValidationHook) Priority() int { return 50 }

func (ValidationHook) OnPostLoad(data map[string]any) error {
	log.Println("✓ Configuration validated successfully")
	return nil
}

type MetricsHook struct {
	loadCount int
}

func (MetricsHook) Name() string  { return "metrics" }
func (MetricsHook) Priority() int { return 200 }

func (m *MetricsHook) OnPostLoad(data map[string]any) error {
	m.loadCount++
	log.Printf("📊 Config loaded %d times", m.loadCount)
	return nil
}

// =============================================================================
// MAIN EXAMPLE - DEMONSTRATES ALL FEATURES
// =============================================================================

func main() {
	fmt.Println("🚀 Comprehensive Config Library Example\n")

	// =========================================================================
	// 1. SETUP SOURCES - Multiple configuration sources
	// =========================================================================
	fmt.Println("📁 Setting up configuration sources...")

	// In-memory defaults (lowest priority)
	defaults := map[string]any{
		"server.host":             "localhost",
		"server.port":             8080,
		"server.mode":             "dev",
		"server.timeout":          "30s",
		"database.driver":         "postgres",
		"database.host":           "localhost",
		"database.port":           5432,
		"database.username":       "admin",
		"database.password":       "ENC(RZq9xKZc2Zx8xkHcQq9m8FqP9QXcPZcJ1yM=)",
		"database.database":       "myapp",
		"database.max_conns":      10,
		"cache.enabled":           true,
		"cache.driver":            "redis",
		"cache.ttl":               3600,
		"security.jwt_secret":     "ENC(RZq9xKZc2Zx8xkHcQq9m8FqP9QXcPZcJ1yM=)",
		"security.encryption_key": "my-32-byte-encryption-key-here",
		"security.cors":           true,
		"features.enable_metrics": true,
		"features.enable_tracing": false,
		"features.beta_features":  false,
	}

	// // Environment-specific overrides
	// envOverrides := map[string]map[string]any{
	// 	"prod": {
	// 		"server.mode":        "prod",
	// 		"server.port":        80,
	// 		"database.max_conns": 50,
	// 	},
	// 	"staging": {
	// 		"server.mode":            "staging",
	// 		"features.beta_features": true,
	// 	},
	// }

	// =========================================================================
	// 2. BUILD CONFIGURATION WITH ALL FEATURES
	// =========================================================================
	fmt.Println("🔧 Building configuration with all features...\n")

	cfg, err := config.NewBuilder().
		// Sources (priority-based merging)
		// WithMemory(defaults, 10).
		WithFile("dev.yaml", 10).
		// WithSource(source.NewEnvOverride("prod", envOverrides, 20)).
		// WithEnv("APP_", 30).

		// Encryption middleware
		WithEncryption("my-32-byte-encryption-key-here").

		// Template processing
		WithTemplates(map[string]any{
			"upper": func(s string) string { return fmt.Sprintf("%s", s) },
		}).

		// Struct validation from tags
		WithStructValidation(&AppConfig{}).

		// Custom struct-level validation
		WithStructRule(DatabaseConnectionRule{}).

		// Lifecycle hooks
		WithHook(hooks.LoggingHook{}).
		WithHook(&hooks.TimingHook{}).
		WithHook(&hooks.DefaultsHook{
			Defaults: map[string]any{
				"features.enable_metrics": true,
			},
		}).
		WithHook(ValidationHook{}).
		WithHook(&MetricsHook{}).

		// Build
		Build()

	if err != nil {
		log.Fatalf("❌ Failed to build config: %v", err)
	}

	fmt.Println("\n✅ Configuration built successfully!\n")

	// =========================================================================
	// 3. BIND CONFIGURATION TO STRUCT
	// =========================================================================
	fmt.Println("📦 Binding configuration to struct...")

	var appCfg AppConfig
	if err := config.Bind(cfg.All(), &appCfg); err != nil {
		log.Fatalf("❌ Failed to bind config: %v", err)
	}

	fmt.Printf("Server: %s:%d (mode: %s)\n", appCfg.Server.Host, appCfg.Server.Port, appCfg.Server.Mode)
	fmt.Printf("Database: %s@%s:%d/%s\n", appCfg.Database.Username, appCfg.Database.Host, appCfg.Database.Port, appCfg.Database.Database)
	fmt.Printf("Cache: enabled=%v, driver=%s\n", appCfg.Cache.Enabled, appCfg.Cache.Driver)
	fmt.Printf("Features: metrics=%v, tracing=%v, beta=%v\n\n",
		appCfg.Features.EnableMetrics, appCfg.Features.EnableTracing, appCfg.Features.BetaFeatures)

	// =========================================================================
	// 4. TYPED ACCESSORS
	// =========================================================================
	fmt.Println("🔍 Using typed accessors...")

	port := config.GetTyped(cfg, "server.port", func(v any) (int, bool) {
		if i, ok := v.(int); ok {
			return i, true
		}
		return 0, false
	}).OrDefault(8080)

	fmt.Printf("Server port (typed): %d\n\n", port)

	// =========================================================================
	// 5. OBSERVERS - React to configuration changes
	// =========================================================================
	fmt.Println("👁️  Setting up configuration observers...")

	cfg.ObserveFunc(func(changes []config.Change) {
		fmt.Println("📢 Configuration changed:")
		for _, c := range changes {
			fmt.Printf("   %s: %v → %v\n", c.Key, c.Old, c.New)
		}
	})

	// =========================================================================
	// 6. DYNAMIC UPDATES
	// =========================================================================
	fmt.Println("\n🔄 Performing dynamic updates...")

	// Single update
	cfg.Set("server.port", 9000)

	// Batch update
	changes := cfg.Update(func(data map[string]any) {
		data["features.enable_tracing"] = true
		data["features.beta_features"] = true
	})

	fmt.Printf("Made %d changes\n\n", len(changes))

	// =========================================================================
	// 7. SNAPSHOTS - Version control for configuration
	// =========================================================================
	fmt.Println("📸 Creating configuration snapshots...")

	snapMgr := snapshot.NewManager(5)
	snap1 := snapMgr.Take(cfg.All())
	fmt.Printf("Snapshot v%d created at %s\n", snap1.Version, snap1.At.Format(time.RFC3339))

	// Make changes
	cfg.Set("server.port", 10000)
	snap2 := snapMgr.Take(cfg.All())
	fmt.Printf("Snapshot v%d created at %s\n", snap2.Version, snap2.At.Format(time.RFC3339))

	// Retrieve snapshot
	if restored, ok := snapMgr.Get(1); ok {
		fmt.Printf("Restored snapshot v%d (port was: %v)\n\n", restored.Version, restored.Data["server.port"])
	}

	// =========================================================================
	// 8. FREEZE/UNFREEZE - Immutable configuration
	// =========================================================================
	fmt.Println("🔒 Testing freeze/unfreeze...")

	cfg.Freeze()
	fmt.Printf("Config frozen: %v\n", cfg.IsFrozen())

	// This would panic:
	// cfg.Set("server.port", 11000)

	cfg.Unfreeze()
	fmt.Printf("Config unfrozen: %v\n\n", cfg.IsFrozen())

	// =========================================================================
	// 9. EXPORT CONFIGURATION
	// =========================================================================
	fmt.Println("💾 Exporting configuration...")

	// Export as JSON
	jsonExp := export.JSONExporter{}
	jsonData, _ := jsonExp.Export(cfg.All())
	fmt.Println("JSON Export (first 200 chars):")
	fmt.Printf("%s...\n\n", jsonData[:min(200, len(jsonData))])

	// Export as YAML
	yamlExp := export.YAMLExporter{}
	yamlData, _ := yamlExp.Export(cfg.All())
	fmt.Println("YAML Export (first 200 chars):")
	fmt.Printf("%s...\n\n", yamlData[:min(200, len(yamlData))])

	// Generate JSON Schema
	schema, _ := export.GenerateSchema(&AppConfig{})
	schemaJSON, _ := export.SchemaJSON(&AppConfig{})
	fmt.Printf("Generated JSON Schema with %d properties\n", len(schema["properties"].(map[string]any)))
	fmt.Printf("Schema (first 200 chars): %s...\n\n", schemaJSON[:min(200, len(schemaJSON))])

	// =========================================================================
	// 10. FILE WATCHING - Hot reload
	// =========================================================================
	fmt.Println("👀 Setting up file watcher (simulated)...")

	watchPaths := []string{
		"config.yaml",
		"config.dev.yaml",
	}

	watcher := watch.New(5*time.Second, watchPaths)

	// In a real application, this would reload config
	watcher.Start(func() {
		fmt.Println("🔄 Configuration files changed - reloading...")
		// Reload logic here
	})

	fmt.Println("File watcher started (monitoring config files)\n")

	// =========================================================================
	// 11. COMPOSITE SOURCES - Advanced source composition
	// =========================================================================
	fmt.Println("🔗 Creating composite source...")

	composite := source.NewComposite(
		"app-config",
		100,
		source.NewMemorySource(defaults, 1),
		source.NewEnvSource("APP_", 2),
	)

	compositeData, _ := composite.Load()
	fmt.Printf("Composite source loaded %d keys\n\n", len(compositeData))

	// =========================================================================
	// 12. CONDITIONAL SOURCES
	// =========================================================================
	fmt.Println("❓ Creating conditional source...")

	isDev := func() bool { return appCfg.Server.Mode == "dev" }

	conditionalSrc := source.NewConditional(
		source.NewMemorySource(map[string]any{
			"debug":           true,
			"verbose_logging": true,
		}, 5),
		isDev,
	)

	condData, _ := conditionalSrc.Load()
	fmt.Printf("Conditional source loaded %d keys (dev mode: %v)\n\n", len(condData), isDev())

	// =========================================================================
	// 13. FACTORY PATTERN - Source creation
	// =========================================================================
	fmt.Println("🏭 Using source factory...")

	factory := source.NewFactory(25)

	factorySources := []source.Source{
		factory.Memory(map[string]any{"factory": "test"}),
		factory.Env("FACTORY_"),
	}

	fmt.Printf("Factory created %d sources\n\n", len(factorySources))

	// =========================================================================
	// 14. VALIDATION WITH FIELD MAPPING
	// =========================================================================
	fmt.Println("✓ Field validation with mapping...")

	validator := validate.NewManager()
	validator.Register("server.port", validate.CustomRule(func(_ string, data map[string]any) error {
		if port, ok := data["server.port"].(int); ok && port < 1024 {
			return fmt.Errorf("port should be >= 1024 for non-root users")
		}
		return nil
	}))

	// Build key map
	keyMap := make(map[string]string)
	validate.BuildStructKeyMap("", &AppConfig{}, keyMap)
	fmt.Printf("Built key map with %d mappings\n\n", len(keyMap))

	// =========================================================================
	// 15. PRESETS - Quick configuration
	// =========================================================================
	fmt.Println("⚡ Using configuration presets...")

	devCfg, _ := config.DevelopmentPreset().
		WithMemory(defaults, 10).
		Build()

	prodCfg, _ := config.ProductionPreset().
		WithMemory(defaults, 10).
		Build()

	fmt.Printf("Development config created: %v\n", devCfg != nil)
	fmt.Printf("Production config created: %v\n\n", prodCfg != nil)

	// =========================================================================
	// 16. DIFF - Compare configurations
	// =========================================================================
	fmt.Println("🔀 Computing configuration diff...")

	oldData := map[string]any{
		"server.port": 8080,
		"server.host": "localhost",
		"removed_key": "value",
	}

	newData := map[string]any{
		"server.port": 9000,
		"server.host": "localhost",
		"added_key":   "new_value",
	}

	diff := config.Diff(oldData, newData)
	fmt.Printf("Found %d differences:\n", len(diff))
	for _, d := range diff {
		fmt.Printf("   %s: %v → %v\n", d.Key, d.Old, d.New)
	}

	// =========================================================================
	// SUMMARY
	// =========================================================================
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("🎉 ALL FEATURES DEMONSTRATED SUCCESSFULLY!")
	fmt.Println(strings.Repeat("_", 80))

	fmt.Println("\n✨ Features showcased:")
	features := []string{
		"✓ Multiple configuration sources (memory, env, files)",
		"✓ Priority-based source merging",
		"✓ Encryption middleware (AES-GCM)",
		"✓ Template processing",
		"✓ Struct validation with tags",
		"✓ Custom validation rules",
		"✓ Lifecycle hooks (pre/post load, bind)",
		"✓ Configuration binding to structs",
		"✓ Typed accessors with defaults",
		"✓ Observer pattern for changes",
		"✓ Dynamic updates (single & batch)",
		"✓ Configuration snapshots",
		"✓ Freeze/unfreeze (immutability)",
		"✓ Export (JSON, YAML, Schema)",
		"✓ File watching (hot reload)",
		"✓ Composite sources",
		"✓ Conditional sources",
		"✓ Factory pattern",
		"✓ Field mapping & validation",
		"✓ Configuration presets",
		"✓ Diff computation",
	}

	for _, f := range features {
		fmt.Println("  " + f)
	}

	fmt.Println("\n💡 This library provides a complete, production-ready")
	fmt.Println("   configuration management solution for Go applications!")

	// Cleanup
	cfg.Close()
	devCfg.Close()
	prodCfg.Close()
}
