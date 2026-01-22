package config

import (
	"context"
	"errors"
	"reflect"
	"sync"
)

// Config is the central, thread-safe configuration container.
type Config struct {
	mu        sync.RWMutex
	data      map[string]any
	frozen    bool
	ctx       context.Context
	cancel    context.CancelFunc
	observers []Observer
}

// NewConfig creates a new Config instance.
func NewConfig() *Config {
	ctx, cancel := context.WithCancel(context.Background())
	return &Config{
		data:      make(map[string]any),
		ctx:       ctx,
		cancel:    cancel,
		observers: make([]Observer, 0),
	}
}

// Close shuts down the configuration context.
func (c *Config) Close() {
	c.cancel()
}

// Freeze prevents further mutations.
func (c *Config) Freeze() {
	c.setFrozenState(true)
}

// Unfreeze allows mutations again.
func (c *Config) Unfreeze() {
	c.setFrozenState(false)
}

func (c *Config) setFrozenState(frozen bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.frozen = frozen
}

// IsFrozen reports whether config is frozen.
func (c *Config) IsFrozen() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.frozen
}

// Get returns a value and existence flag.
func (c *Config) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.data[key]
	return v, ok
}

// Set sets a value (panic if frozen).
func (c *Config) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checkFrozen()
	c.data[key] = value
}

func (c *Config) checkFrozen() {
	if c.frozen {
		panic(errors.New("config is frozen"))
	}
}

// All returns a shallow copy of config data.
func (c *Config) All() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return copyData(c.data)
}

func copyData(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// Update performs an atomic batch update.
func (c *Config) Update(fn func(map[string]any)) []Change {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.checkFrozen()
	old := copyData(c.data)

	fn(c.data)
	changes := Diff(old, c.data)

	go notifyObservers(c.observers, changes)
	return changes
}

// Load replaces the entire configuration atomically.
func (c *Config) Load(newData map[string]any) []Change {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.checkFrozen()
	changes := c.replaceData(newData)

	go notifyObservers(c.observers, changes)
	return changes
}

func (c *Config) replaceData(newData map[string]any) []Change {
	old := c.data
	c.data = newData
	return Diff(old, newData)
}

// Observe registers an observer.
func (c *Config) Observe(o Observer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.observers = append(c.observers, o)
}

// ObserveFunc registers a function observer.
func (c *Config) ObserveFunc(fn func([]Change)) {
	c.Observe(ObserverFunc(fn))
}

// Optional provides fluent default handling.
type Optional[T any] struct {
	value T
	ok    bool
}

// OrDefault returns value or fallback.
func (o Optional[T]) OrDefault(v T) T {
	if o.ok {
		return o.value
	}
	return v
}

// GetTyped retrieves and converts a value.
func GetTyped[T any](c *Config, key string, conv func(any) (T, bool)) Optional[T] {
	v, ok := c.Get(key)
	if !ok {
		return Optional[T]{}
	}

	return convertValue(v, conv)
}

func convertValue[T any](v any, conv func(any) (T, bool)) Optional[T] {
	if out, ok := conv(v); ok {
		return Optional[T]{value: out, ok: true}
	}
	return Optional[T]{}
}

// Change represents a single config mutation.
type Change struct {
	Key string
	Old any
	New any
}

// Diff computes differences between two maps.
func Diff(old, newData map[string]any) []Change {
	changes := make([]Change, 0)

	changes = appendRemovedAndModified(changes, old, newData)
	changes = appendAdded(changes, old, newData)

	return changes
}

func appendRemovedAndModified(changes []Change, old, newData map[string]any) []Change {
	for k, ov := range old {
		nv, exists := newData[k]
		if !exists {
			changes = append(changes, Change{Key: k, Old: ov, New: nil})
			continue
		}
		if !reflect.DeepEqual(ov, nv) {
			changes = append(changes, Change{Key: k, Old: ov, New: nv})
		}
	}
	return changes
}

func appendAdded(changes []Change, old, newData map[string]any) []Change {
	for k, nv := range newData {
		if _, exists := old[k]; !exists {
			changes = append(changes, Change{Key: k, Old: nil, New: nv})
		}
	}
	return changes
}

// Observer reacts to config changes.
type Observer interface {
	OnChange([]Change)
}

// ObserverFunc adapts a function.
type ObserverFunc func([]Change)

func (f ObserverFunc) OnChange(c []Change) { f(c) }

func notifyObservers(obs []Observer, c []Change) {
	for _, o := range obs {
		o.OnChange(c)
	}
}
