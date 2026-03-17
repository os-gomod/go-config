// Package config_test provides comprehensive tests for the configuration library.
package config_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/os-gomod/go-config"
	"github.com/os-gomod/go-config/types"
	"github.com/os-gomod/go-config/validate"
)

// TestNew tests creating a new configuration instance.
func TestNew(t *testing.T) {
	cfg, err := config.New()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	defer cfg.Close(context.Background())

	if cfg == nil {
		t.Fatal("Config should not be nil")
	}
}

// TestWithMemory tests creating configuration with in-memory data.
func TestWithMemory(t *testing.T) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"key1": "value1",
			"key2": 42,
			"key3": true,
		}),
	)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	defer cfg.Close(context.Background())

	if v := cfg.GetString("key1"); v != "value1" {
		t.Errorf("Expected value1, got %s", v)
	}

	if v := cfg.GetInt("key2"); v != 42 {
		t.Errorf("Expected 42, got %d", v)
	}

	if v := cfg.GetBool("key3"); !v {
		t.Errorf("Expected true, got %v", v)
	}
}

// TestWithEnv tests loading configuration from environment variables.
func TestWithEnv(t *testing.T) {
	os.Setenv("TEST_KEY1", "value1")
	os.Setenv("TEST_KEY2", "42")
	os.Setenv("TEST_KEY3", "true")
	defer func() {
		os.Unsetenv("TEST_KEY1")
		os.Unsetenv("TEST_KEY2")
		os.Unsetenv("TEST_KEY3")
	}()

	cfg, err := config.New(config.WithEnv("TEST_"))
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	defer cfg.Close(context.Background())

	if v := cfg.GetString("key1"); v != "value1" {
		t.Errorf("Expected value1, got %s", v)
	}

	if v := cfg.GetInt("key2"); v != 42 {
		t.Errorf("Expected 42, got %d", v)
	}

	if v := cfg.GetBool("key3"); !v {
		t.Errorf("Expected true, got %v", v)
	}
}

// TestGet tests value retrieval.
func TestGet(t *testing.T) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"string":  "value",
			"int":     42,
			"float":   3.14,
			"bool":    true,
			"missing": nil,
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer cfg.Close(context.Background())

	// Test string
	if v := cfg.GetString("string"); v != "value" {
		t.Errorf("Expected value, got %s", v)
	}

	// Test int
	if v := cfg.GetInt("int"); v != 42 {
		t.Errorf("Expected 42, got %d", v)
	}

	// Test float
	if v := cfg.GetFloat64("float"); v != 3.14 {
		t.Errorf("Expected 3.14, got %f", v)
	}

	// Test bool
	if v := cfg.GetBool("bool"); !v {
		t.Errorf("Expected true, got %v", v)
	}

	// Test missing key
	if v := cfg.GetString("nonexistent"); v != "" {
		t.Errorf("Expected empty string, got %s", v)
	}
}

// TestGetDefault tests value retrieval with defaults.
func TestGetDefault(t *testing.T) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"existing": "value",
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer cfg.Close(context.Background())

	if v := cfg.GetStringDefault("existing", "default"); v != "value" {
		t.Errorf("Expected value, got %s", v)
	}

	if v := cfg.GetStringDefault("nonexistent", "default"); v != "default" {
		t.Errorf("Expected default, got %s", v)
	}

	if v := cfg.GetIntDefault("nonexistent", 100); v != 100 {
		t.Errorf("Expected 100, got %d", v)
	}

	if v := cfg.GetBoolDefault("nonexistent", true); !v {
		t.Errorf("Expected true, got %v", v)
	}
}

// TestSet tests setting values.
func TestSet(t *testing.T) {
	cfg, err := config.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cfg.Close(context.Background())

	// Set string
	if setErr := cfg.Set("key", "value"); setErr != nil {
		t.Fatalf("Failed to set: %v", setErr)
	}

	if v := cfg.GetString("key"); v != "value" {
		t.Errorf("Expected value, got %s", v)
	}

	// Overwrite
	if setErr := cfg.Set("key", "newvalue"); setErr != nil {
		t.Fatalf("Failed to overwrite: %v", setErr)
	}

	if v := cfg.GetString("key"); v != "newvalue" {
		t.Errorf("Expected newvalue, got %s", v)
	}
}

// TestDelete tests deleting values.
func TestDelete(t *testing.T) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"key": "value",
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer cfg.Close(context.Background())

	if !cfg.Has("key") {
		t.Fatal("Key should exist")
	}

	if setErr := cfg.Delete("key"); setErr != nil {
		t.Fatalf("Failed to delete: %v", setErr)
	}

	if cfg.Has("key") {
		t.Fatal("Key should be deleted")
	}
}

// TestBind tests struct binding.
func TestBind(t *testing.T) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"server.host":          "localhost",
			"server.port":          8080,
			"server.read_timeout":  "30s",
			"server.write_timeout": "60s",
			"database.host":        "db.example.com",
			"database.port":        5432,
			"database.name":        "mydb",
			"database.user":        "admin",
			"database.password":    "secret",
			"database.max_conns":   100,
			"logging.level":        "info",
			"logging.format":       "json",
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer cfg.Close(context.Background())

	var appCfg AppConfig
	if setErr := cfg.Bind(&appCfg); setErr != nil {
		t.Fatalf("Failed to bind: %v", setErr)
	}

	if appCfg.Server.Host != "localhost" {
		t.Errorf("Expected localhost, got %s", appCfg.Server.Host)
	}

	if appCfg.Server.Port != 8080 {
		t.Errorf("Expected 8080, got %d", appCfg.Server.Port)
	}

	if appCfg.Server.ReadTimeout != 30*time.Second {
		t.Errorf("Expected 30s, got %v", appCfg.Server.ReadTimeout)
	}

	if appCfg.Database.Host != "db.example.com" {
		t.Errorf("Expected db.example.com, got %s", appCfg.Database.Host)
	}

	if appCfg.Database.MaxConns != 100 {
		t.Errorf("Expected 100, got %d", appCfg.Database.MaxConns)
	}
}

// TestSnapshot tests snapshot functionality.
func TestSnapshot(t *testing.T) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"key": "original",
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer cfg.Close(context.Background())

	// Take snapshot
	snap := cfg.Snapshot()
	if snap == nil {
		t.Fatal("Snapshot should not be nil")
	}

	// Modify config
	cfg.Set("key", "modified")
	if v := cfg.GetString("key"); v != "modified" {
		t.Errorf("Expected modified, got %s", v)
	}

	// Restore from snapshot
	if setErr := cfg.Restore(snap); setErr != nil {
		t.Fatalf("Failed to restore: %v", setErr)
	}

	if v := cfg.GetString("key"); v != "original" {
		t.Errorf("Expected original, got %s", v)
	}
}

// TestExport tests export functionality.
func TestExport(t *testing.T) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"server.host": "localhost",
			"server.port": 8080,
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer cfg.Close(context.Background())

	// Test JSON export
	jsonData, err := cfg.ExportJSON()
	if err != nil {
		t.Fatalf("Failed to export JSON: %v", err)
	}

	if len(jsonData) == 0 {
		t.Error("JSON export should not be empty")
	}

	// Test YAML export
	yamlData, err := cfg.ExportYAML()
	if err != nil {
		t.Fatalf("Failed to export YAML: %v", err)
	}

	if len(yamlData) == 0 {
		t.Error("YAML export should not be empty")
	}
}

// TestValidate tests validation functionality.
func TestValidate(t *testing.T) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"server.port": 8080,
			"log.level":   "info",
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer cfg.Close(context.Background())

	plan := validate.NewBuilder().
		Required("server.host", "server host is required").
		Range("server.port", 1, 65535, "port must be valid").
		Build()

	err1 := cfg.Validate(plan)
	if err1 == nil {
		t.Error("Validation should fail for missing required field")
	}
}

// TestValidateRequiredPresent ensures required-only rules do not panic when
// the key exists.
func TestValidateRequiredPresent(t *testing.T) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"server.host": "localhost",
			"server.port": 8080,
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer cfg.Close(context.Background())

	plan := validate.NewBuilder().
		Required("server.host", "server host is required").
		Range("server.port", 1, 65535, "port must be valid").
		Build()

	if setErr := cfg.Validate(plan); setErr != nil {
		t.Fatalf("Validation should pass, got: %v", setErr)
	}
}

// TestObserver tests observer functionality.
func TestObserver(t *testing.T) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer cfg.Close(context.Background())

	received := make(chan types.Event, 1)

	cfg.Observe(func(_ context.Context, event types.Event) {
		received <- event
	})

	cfg.Set("key", "value")

	select {
	case event := <-received:
		if event.Key != "key" {
			t.Errorf("Expected key, got %s", event.Key)
		}
	case <-time.After(time.Second):
		t.Error("Should have received event")
	}
}

// TestMerge tests configuration merging.
func TestMerge(t *testing.T) {
	base, err := config.New(
		config.WithMemory(map[string]any{
			"key1": "value1",
			"key2": "original",
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close(context.Background())

	override, err := config.New(
		config.WithMemory(map[string]any{
			"key2": "overridden",
			"key3": "value3",
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer override.Close(context.Background())

	if setErr := base.Merge(override); setErr != nil {
		t.Fatalf("Failed to merge: %v", setErr)
	}

	if v := base.GetString("key1"); v != "value1" {
		t.Errorf("Expected value1, got %s", v)
	}

	if v := base.GetString("key2"); v != "overridden" {
		t.Errorf("Expected overridden, got %s", v)
	}

	if v := base.GetString("key3"); v != "value3" {
		t.Errorf("Expected value3, got %s", v)
	}
}

// TestClone tests configuration cloning.
func TestClone(t *testing.T) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"key": "value",
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer cfg.Close(context.Background())

	clone := cfg.Clone()
	if clone == nil {
		t.Fatal("Clone should not be nil")
	}

	// Modify original
	cfg.Set("key", "modified")

	// Clone should have original value
	if v := clone.GetString("key"); v != "value" {
		t.Errorf("Clone should have original value, got %s", v)
	}
}

// TestKeys tests key listing.
func TestKeys(t *testing.T) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"key1": 1,
			"key2": 2,
			"key3": 3,
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer cfg.Close(context.Background())

	keys := cfg.Keys()
	if len(keys) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(keys))
	}
}

// TestHas tests key existence check.
func TestHas(t *testing.T) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"existing": "value",
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer cfg.Close(context.Background())

	if !cfg.Has("existing") {
		t.Error("Key should exist")
	}

	if cfg.Has("nonexistent") {
		t.Error("Key should not exist")
	}
}

// TestClose tests configuration closing.
func TestClose(t *testing.T) {
	cfg, err := config.New()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if setErr := cfg.Close(ctx); setErr != nil {
		t.Fatalf("Failed to close: %v", setErr)
	}

	// Operations on closed config should fail
	if setErr := cfg.Set("key", "value"); setErr == nil {
		t.Error("Set should fail on closed config")
	}
}

// TestContext tests context-aware operations.
func TestContext(t *testing.T) {
	cfg, err := config.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cfg.Close(context.Background())

	ctx := config.ContextWithConfig(context.Background(), cfg)

	retrieved := config.ConfigFromContext(ctx)
	if retrieved == nil {
		t.Error("Should retrieve config from context")
	}
}

// TestStats tests statistics retrieval.
func TestStats(t *testing.T) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"key1": 1,
			"key2": 2,
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer cfg.Close(context.Background())

	stats := cfg.Stats()
	if stats.KeyCount != 2 {
		t.Errorf("Expected 2 keys, got %d", stats.KeyCount)
	}
}

// TestEncryption tests encryption functionality.
func TestEncryption(t *testing.T) {
	key := []byte("32-byte-encryption-key-here!!")

	cfg, err := config.New(
		config.WithMemory(map[string]any{}),
		config.WithEncryption(key),
	)
	if err != nil {
		t.Fatalf("Failed to create config with encryption: %v", err)
	}
	defer cfg.Close(context.Background())

	plaintext := "my-secret-value"
	encrypted, err := cfg.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	if encrypted.Ciphertext == "" {
		t.Error("Ciphertext should not be empty")
	}

	decrypted, err := cfg.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("Expected %s, got %s", plaintext, decrypted)
	}
}

// TestThreadSafety tests concurrent access.
func TestThreadSafety(t *testing.T) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"counter": 0,
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer cfg.Close(context.Background())

	done := make(chan bool)

	// Concurrent reads
	for range 10 {
		go func() {
			for range 100 {
				_ = cfg.GetInt("counter")
			}
			done <- true
		}()
	}

	// Concurrent writes
	for range 5 {
		go func() {
			for j := range 50 {
				cfg.Set("counter", j)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for range 15 {
		<-done
	}
}

// TestNestedKeys tests nested key access.
func TestNestedKeys(t *testing.T) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"server": map[string]any{
				"host": "localhost",
				"port": 8080,
				"tls": map[string]any{
					"enabled": true,
					"cert":    "/path/to/cert",
				},
			},
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer cfg.Close(context.Background())

	if v := cfg.GetString("server.host"); v != "localhost" {
		t.Errorf("Expected localhost, got %s", v)
	}

	if v := cfg.GetInt("server.port"); v != 8080 {
		t.Errorf("Expected 8080, got %d", v)
	}

	if v := cfg.GetBool("server.tls.enabled"); !v {
		t.Errorf("Expected true, got %v", v)
	}
}
