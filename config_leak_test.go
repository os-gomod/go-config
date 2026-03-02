package config_test

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/os-gomod/go-config"
)

func TestLeak(t *testing.T) {
	before := runtime.NumGoroutine()

	cfg, err := config.New(
		config.WithMemory(map[string]any{
			"server.port": 8080,
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Start watch (if supported)
	go func() {
		_ = cfg.Watch(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	cancel()
	cfg.Close(context.Background())

	time.Sleep(200 * time.Millisecond)

	after := runtime.NumGoroutine()

	if after > before+1 { // allow small fluctuation
		t.Fatalf("possible goroutine leak: before=%d after=%d", before, after)
	}
}