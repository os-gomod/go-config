// Package core provides the central state management engine for the configuration system.
// It implements copy-on-write semantics with atomic pointer swaps for lock-free reads.
package core

import (
	"context"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/os-gomod/go-config/types"
)

// State represents an immutable configuration state.
// All fields are read-only after creation to ensure thread-safety.
type State struct {
	// data contains the flattened key-value configuration data.
	data map[string]types.Value

	// indexed contains nested structure representation for binding.
	indexed map[string]any

	// metadata contains additional state information.
	metadata *StateMetadata

	// createdAt is the timestamp when this state was created.
	createdAt time.Time

	// version is incremented on each state change.
	version uint64
}

// StateMetadata contains additional metadata about the state.
type StateMetadata struct {
	sources   []types.SourceType
	encrypted map[string]bool
}

// NewState creates a new immutable configuration state.
func NewState(data map[string]types.Value, opts ...StateOption) *State {
	s := &State{
		data:      data,
		indexed:   make(map[string]any),
		createdAt: time.Now(),
		version:   1,
		metadata: &StateMetadata{
			sources:   make([]types.SourceType, 0),
			encrypted: make(map[string]bool),
		},
	}

	for _, opt := range opts {
		opt(s)
	}

	// Build indexed representation
	s.buildIndexed()

	return s
}

// StateOption configures state creation.
type StateOption func(*State)

// WithVersion sets the state version.
func WithVersion(v uint64) StateOption {
	return func(s *State) { s.version = v }
}

// WithMetadata sets the state metadata.
func WithMetadata(m *StateMetadata) StateOption {
	return func(s *State) { s.metadata = m }
}

// Get retrieves a value by key. Returns the value and existence flag.
// This operation is O(1) and lock-free.
func (s *State) Get(key string) (types.Value, bool) {
	if s == nil || s.data == nil {
		return types.Value{}, false
	}
	v, ok := s.data[key]

	return v, ok
}

// GetAll returns a copy of all configuration data.
// This operation allocates a new map to prevent mutation.
func (s *State) GetAll() map[string]types.Value {
	if s == nil || s.data == nil {
		return nil
	}
	result := make(map[string]types.Value, len(s.data))
	for k, v := range s.data {
		result[k] = v
	}

	return result
}

func (s *State) Export() map[string]any {
	if s == nil || s.data == nil {
		return nil
	}
	result := make(map[string]any, len(s.data))
	for k, v := range s.data {
		result[k] = v.Any()
	}

	return result
}

// Keys returns all keys in the state.
func (s *State) Keys() []string {
	if s == nil || s.data == nil {
		return nil
	}
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}

	return keys
}

// Indexed returns the nested structure representation.
func (s *State) Indexed() map[string]any {
	return s.indexed
}

// Metadata returns the state metadata.
func (s *State) Metadata() *StateMetadata {
	return s.metadata
}

// Version returns the state version.
func (s *State) Version() uint64 {
	return s.version
}

// CreatedAt returns the state creation timestamp.
func (s *State) CreatedAt() time.Time {
	return s.createdAt
}

// buildIndexed converts flattened keys to nested structure.
func (s *State) buildIndexed() {
	for key, value := range s.data {
		s.insertIndexed(key, value.Raw())
	}
}

// insertIndexed inserts a value into the nested structure.
func (s *State) insertIndexed(key string, value any) {
	parts := splitKey(key)
	current := s.indexed

	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = value

			return
		}

		if _, exists := current[part]; !exists {
			current[part] = make(map[string]any)
		}

		if next, ok := current[part].(map[string]any); ok {
			current = next
		} else {
			// Replace non-map with map if there are more parts
			newMap := make(map[string]any)
			current[part] = newMap
			current = newMap
		}
	}
}

// splitKey splits a dotted key into parts.
func splitKey(key string) []string {
	parts := make([]string, 0, 8)
	start := 0
	for i := range len(key) {
		if key[i] == '.' {
			if i > start {
				parts = append(parts, key[start:i])
			}
			start = i + 1
		}
	}
	if start < len(key) {
		parts = append(parts, key[start:])
	}

	return parts
}

// Engine manages configuration state with atomic updates.
type Engine struct {
	// state is the current configuration state, accessed atomically.
	state atomic.Pointer[State]

	// hooks contains lifecycle hooks organized by type.
	hooks map[types.HookType][]types.Hook

	// hooksMu protects hooks map modifications.
	hooksMu sync.RWMutex

	// observerManager handles event distribution.
	observers *ObserverManager

	// closed indicates if the engine is shutting down.
	closed atomic.Bool

	// closeMu protects shutdown operations.
	closeMu sync.Mutex

	// closeChan signals engine shutdown.
	closeChan chan struct{}
}

// NewEngine creates a new configuration engine.
func NewEngine() *Engine {
	e := &Engine{
		hooks:     make(map[types.HookType][]types.Hook),
		closeChan: make(chan struct{}),
		observers: NewObserverManager(100), // Default queue size
	}

	// Initialize with empty state
	e.state.Store(NewState(nil))

	return e
}

// State returns the current configuration state.
// This operation is lock-free and zero-allocation.
func (e *Engine) State() *State {
	return e.state.Load()
}

// Update atomically swaps the configuration state.
// It uses copy-on-write semantics to ensure no race conditions.
func (e *Engine) Update(ctx context.Context, newstate *State) error {
	if e.closed.Load() {
		return types.NewError(types.ErrContextCanceled, "engine is closed")
	}

	// Execute before hooks
	if err := e.executeHooks(ctx, types.HookBeforeSet); err != nil {
		return err
	}

	// Get old state for event generation
	oldState := e.state.Load()

	// Increment version
	newstate.version = oldState.Version() + 1

	// Atomic swap
	e.state.Store(newstate)

	// Generate events for observers
	e.generateEvents(ctx, oldState, newstate)

	// Execute after hooks
	return e.executeHooks(ctx, types.HookAfterSet)
}

// Load loads new state from multiple sources with priority merging.
func (e *Engine) Load(ctx context.Context, sources []Source) error {
	if e.closed.Load() {
		return types.NewError(types.ErrContextCanceled, "engine is closed")
	}

	// Execute before load hooks
	if err := e.executeHooks(ctx, types.HookBeforeLoad); err != nil {
		return err
	}

	// Load from all sources
	allData := make(map[string]types.Value)
	for _, source := range sources {
		data, err := source.Load(ctx)
		if err != nil {
			return err
		}

		// Merge with priority handling
		for k, v := range data {
			if existing, ok := allData[k]; !ok || v.Priority() > existing.Priority() {
				allData[k] = v
			}
		}
	}

	// Create new state
	newState := NewState(allData)

	// Atomic update
	if err := e.Update(ctx, newState); err != nil {
		return err
	}

	// Execute after load hooks
	return e.executeHooks(ctx, types.HookAfterLoad)
}

// RegisterHook adds a lifecycle hook.
func (e *Engine) RegisterHook(hookType types.HookType, hook types.Hook) {
	e.hooksMu.Lock()
	defer e.hooksMu.Unlock()

	e.hooks[hookType] = append(e.hooks[hookType], hook)
}

// executeHooks runs all hooks of a given type.
func (e *Engine) executeHooks(ctx context.Context, hookType types.HookType) error {
	e.hooksMu.RLock()
	hooks := e.hooks[hookType]
	e.hooksMu.RUnlock()

	for _, hook := range hooks {
		if err := hook(ctx); err != nil {
			return err
		}
	}

	return nil
}

// generateEvents creates events for state changes and notifies observers.
func (e *Engine) generateEvents(ctx context.Context, oldState, newState *State) {
	if oldState == nil || newState == nil {
		return
	}

	now := time.Now()

	// Check for create and update events
	for key, newVal := range newState.data {
		if oldVal, exists := oldState.data[key]; exists {
			// Update event
			if !valuesEqual(oldVal, newVal) {
				e.observers.Notify(ctx, types.Event{
					Type:      types.EventUpdate,
					Key:       key,
					OldValue:  oldVal,
					NewValue:  newVal,
					Timestamp: now,
					Source:    newVal.Source(),
				})
			}
		} else {
			// Create event
			e.observers.Notify(ctx, types.Event{
				Type:      types.EventCreate,
				Key:       key,
				NewValue:  newVal,
				Timestamp: now,
				Source:    newVal.Source(),
			})
		}
	}

	// Check for delete events
	for key, oldVal := range oldState.data {
		if _, exists := newState.data[key]; !exists {
			e.observers.Notify(ctx, types.Event{
				Type:      types.EventDelete,
				Key:       key,
				OldValue:  oldVal,
				Timestamp: now,
				Source:    oldVal.Source(),
			})
		}
	}
}

// valuesEqual compares two values for equality.
func valuesEqual(a, b types.Value) bool {
	return reflect.DeepEqual(a.Raw(), b.Raw()) && a.Type() == b.Type()
}

// Observe registers an observer for configuration events.
func (e *Engine) Observe(ctx context.Context, observer types.Observer) (func(), error) {
	return e.observers.Subscribe(ctx, observer)
}

// Close shuts down the engine gracefully.
func (e *Engine) Close(ctx context.Context) error {
	e.closeMu.Lock()
	defer e.closeMu.Unlock()

	if e.closed.Swap(true) {
		return nil // Already closed
	}

	close(e.closeChan)

	// Close observer manager
	return e.observers.Close(ctx)
}

// Closed returns true if the engine is shut down.
func (e *Engine) Closed() bool {
	return e.closed.Load()
}

// Done returns a channel that's closed when the engine shuts down.
func (e *Engine) Done() <-chan struct{} {
	return e.closeChan
}

// Source defines the interface for configuration sources.
type Source interface {
	// Load reads configuration from the source.
	Load(ctx context.Context) (map[string]types.Value, error)

	// Name returns the source name for identification.
	Name() string

	// Priority returns the source priority for merge ordering.
	Priority() int

	// Close releases resources associated with the source.
	Close(ctx context.Context) error
}
