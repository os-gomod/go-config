package hooks

import (
	"log"
	"time"
)

// LoggingHook logs lifecycle events.
type LoggingHook struct{}

func (LoggingHook) Name() string  { return "logging" }
func (LoggingHook) Priority() int { return 100 }

func (LoggingHook) OnPreLoad(_ map[string]any) error {
	log.Println("config: loading")
	return nil
}

func (LoggingHook) OnPostLoad(_ map[string]any) error {
	log.Println("config: loaded")
	return nil
}

// DefaultsHook applies default values.
type DefaultsHook struct {
	Defaults map[string]any
}

func (DefaultsHook) Name() string  { return "defaults" }
func (DefaultsHook) Priority() int { return 10 }

func (h DefaultsHook) OnPostLoad(data map[string]any) error {
	applyDefaults(data, h.Defaults)
	return nil
}

func applyDefaults(data, defaults map[string]any) {
	for k, v := range defaults {
		if _, exists := data[k]; !exists {
			data[k] = v
		}
	}
}

// TimingHook measures load time.
type TimingHook struct {
	start time.Time
}

func (TimingHook) Name() string  { return "timing" }
func (TimingHook) Priority() int { return 1000 }

func (h *TimingHook) OnPreLoad(_ map[string]any) error {
	h.start = time.Now()
	return nil
}

func (h *TimingHook) OnPostLoad(_ map[string]any) error {
	elapsed := time.Since(h.start)
	log.Println("config load took", elapsed)
	return nil
}
