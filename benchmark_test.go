// Package config_test provides benchmarks for the configuration library.
package config_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/os-gomod/go-config"
	"github.com/os-gomod/go-config/types"
)

// BenchmarkGet benchmarks configuration value retrieval.
// Target: < 50ns, zero allocations.
func BenchmarkGet(b *testing.B) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"server.port":    8080,
			"server.host":    "localhost",
			"database.name":  "mydb",
			"database.host":  "localhost",
			"database.port":  5432,
			"logging.level":  "info",
			"logging.format": "json",
		}),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer cfg.Close(context.Background())

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = cfg.GetInt("server.port")
			_ = cfg.GetString("server.host")
			_ = cfg.GetInt("database.port")
		}
	})
}

// BenchmarkGetSingle benchmarks single key retrieval.
func BenchmarkGetSingle(b *testing.B) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"key": "value",
		}),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer cfg.Close(context.Background())

	b.ResetTimer()
	for range b.N {
		_, _ = cfg.Get("key")
	}
}

// BenchmarkGetString benchmarks string retrieval.
func BenchmarkGetString(b *testing.B) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"key": "value",
		}),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer cfg.Close(context.Background())

	b.ResetTimer()
	for range b.N {
		_ = cfg.GetString("key")
	}
}

// BenchmarkGetInt benchmarks int retrieval.
func BenchmarkGetInt(b *testing.B) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"key": 42,
		}),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer cfg.Close(context.Background())

	b.ResetTimer()
	for range b.N {
		_ = cfg.GetInt("key")
	}
}

// BenchmarkGetBool benchmarks bool retrieval.
func BenchmarkGetBool(b *testing.B) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"key": true,
		}),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer cfg.Close(context.Background())

	b.ResetTimer()
	for range b.N {
		_ = cfg.GetBool("key")
	}
}

// BenchmarkSet benchmarks configuration value updates.
// Target: < 500µs, minimal allocations.
func BenchmarkSet(b *testing.B) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{}),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer cfg.Close(context.Background())

	b.ResetTimer()
	for i := range b.N {
		_ = cfg.Set("key", i)
	}
}

// BenchmarkSetParallel benchmarks parallel configuration updates.
func BenchmarkSetParallel(b *testing.B) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{}),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer cfg.Close(context.Background())

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_ = cfg.Set("key", i)
			i++
		}
	})
}

// BenchmarkBind benchmarks struct binding.
func BenchmarkBind(b *testing.B) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"server.host":          "localhost",
			"server.port":          8080,
			"server.read_timeout":  "30s",
			"server.write_timeout": "30s",
			"database.host":        "localhost",
			"database.port":        5432,
			"database.name":        "mydb",
			"database.user":        "user",
			"database.password":    "pass",
			"database.max_conns":   100,
			"logging.level":        "info",
			"logging.format":       "json",
		}),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer cfg.Close(context.Background())

	b.ResetTimer()
	for range b.N {
		var appCfg AppConfig
		_ = cfg.Bind(&appCfg)
	}
}

// BenchmarkBindCached benchmarks struct binding with cached metadata.
func BenchmarkBindCached(b *testing.B) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"server.host":   "localhost",
			"server.port":   8080,
			"database.host": "localhost",
			"database.port": 5432,
			"database.name": "mydb",
		}),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer cfg.Close(context.Background())

	// First bind to cache metadata
	var appCfg AppConfig
	_ = cfg.Bind(&appCfg)

	b.ResetTimer()
	for range b.N {
		var newCfg AppConfig
		_ = cfg.Bind(&newCfg)
	}
}

// BenchmarkSnapshot benchmarks snapshot creation.
func BenchmarkSnapshot(b *testing.B) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		}),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer cfg.Close(context.Background())

	b.ResetTimer()
	for range b.N {
		_ = cfg.Snapshot()
	}
}

// BenchmarkExportJSON benchmarks JSON export.
func BenchmarkExportJSON(b *testing.B) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"server.host": "localhost",
			"server.port": 8080,
			"database": map[string]any{
				"host": "localhost",
				"port": 5432,
				"name": "mydb",
			},
		}),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer cfg.Close(context.Background())

	b.ResetTimer()
	for range b.N {
		_, _ = cfg.ExportJSON()
	}
}

// BenchmarkExportYAML benchmarks YAML export.
func BenchmarkExportYAML(b *testing.B) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"server.host": "localhost",
			"server.port": 8080,
			"database": map[string]any{
				"host": "localhost",
				"port": 5432,
				"name": "mydb",
			},
		}),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer cfg.Close(context.Background())

	b.ResetTimer()
	for range b.N {
		_, _ = cfg.ExportYAML()
	}
}

// BenchmarkObserver benchmarks observer notifications.
func BenchmarkObserver(b *testing.B) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{}),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer cfg.Close(context.Background())

	var received atomic.Int64
	cfg.Observe(func(_ context.Context, _ types.Event) {
		received.Add(1)
	})

	b.ResetTimer()
	for i := range b.N {
		_ = cfg.Set("key", i)
	}
}

// BenchmarkMerge benchmarks configuration merging.
func BenchmarkMerge(b *testing.B) {
	base, err := config.New(
		config.WithMemory(map[string]any{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		}),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer base.Close(context.Background())

	override, err := config.New(
		config.WithMemory(map[string]any{
			"key2": "newvalue2",
			"key4": "value4",
		}),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer override.Close(context.Background())

	b.ResetTimer()
	for range b.N {
		_ = base.Merge(override)
	}
}

// BenchmarkClone benchmarks configuration cloning.
func BenchmarkClone(b *testing.B) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"server.host": "localhost",
			"server.port": 8080,
		}),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer cfg.Close(context.Background())

	b.ResetTimer()
	for range b.N {
		_ = cfg.Clone()
	}
}

// BenchmarkKeys benchmarks key listing.
func BenchmarkKeys(b *testing.B) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"key1": 1,
			"key2": 2,
			"key3": 3,
			"key4": 4,
			"key5": 5,
		}),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer cfg.Close(context.Background())

	b.ResetTimer()
	for range b.N {
		_ = cfg.Keys()
	}
}

// BenchmarkHas benchmarks key existence check.
func BenchmarkHas(b *testing.B) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"existing": "value",
		}),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer cfg.Close(context.Background())

	b.ResetTimer()
	for range b.N {
		_ = cfg.Has("existing")
		_ = cfg.Has("nonexistent")
	}
}

// BenchmarkWithLargeConfig benchmarks operations with large configuration.
func BenchmarkWithLargeConfig(b *testing.B) {
	// Create large configuration
	data := make(map[string]any)
	for i := range 1000 {
		data[fmt.Sprintf("key%d", i)] = fmt.Sprintf("value%d", i)
	}

	cfg, err := config.New(config.WithMemory(data))
	if err != nil {
		b.Fatal(err)
	}
	defer cfg.Close(context.Background())

	b.Run("Get", func(b *testing.B) {
		b.ResetTimer()
		for range b.N {
			_ = cfg.GetString("key500")
		}
	})

	b.Run("Set", func(b *testing.B) {
		b.ResetTimer()
		for range b.N {
			_ = cfg.Set("key500", "newvalue")
		}
	})

	b.Run("Keys", func(b *testing.B) {
		b.ResetTimer()
		for range b.N {
			_ = cfg.Keys()
		}
	})
}

// BenchmarkConcurrentReadWrite benchmarks concurrent reads and writes.
func BenchmarkConcurrentReadWrite(b *testing.B) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"key": "initial",
		}),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer cfg.Close(context.Background())

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%10 == 0 {
				// 10% writes
				_ = cfg.Set("key", i)
			} else {
				// 90% reads
				_ = cfg.GetString("key")
			}
			i++
		}
	})
}

// BenchmarkEncrypt benchmarks encryption.
func BenchmarkEncrypt(b *testing.B) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{}),
		config.WithEncryption([]byte("32-byte-encryption-key-here!!")),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer cfg.Close(context.Background())

	b.ResetTimer()
	for range b.N {
		_, _ = cfg.Encrypt("my-secret-value")
	}
}

// BenchmarkDecrypt benchmarks decryption.
func BenchmarkDecrypt(b *testing.B) {
	cfg, err := config.New(
		config.WithMemory(map[string]any{}),
		config.WithEncryption([]byte("32-byte-encryption-key-here!!")),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer cfg.Close(context.Background())

	encrypted, err1 := cfg.Encrypt("my-secret-value")
	if err1 != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for range b.N {
		_, _ = cfg.Decrypt(encrypted)
	}
}
