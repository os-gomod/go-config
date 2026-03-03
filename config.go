// Package config provides a production-grade configuration management library.
// It offers zero-allocation reads, atomic updates, and thread-safe operations.
package config

import (
	"context"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/os-gomod/go-config/bind"
	"github.com/os-gomod/go-config/core"
	"github.com/os-gomod/go-config/crypto"
	"github.com/os-gomod/go-config/export"
	"github.com/os-gomod/go-config/merge"
	"github.com/os-gomod/go-config/parser"
	"github.com/os-gomod/go-config/snapshot"
	"github.com/os-gomod/go-config/source"
	"github.com/os-gomod/go-config/types"
	"github.com/os-gomod/go-config/validate"
	"github.com/os-gomod/go-config/watch"
)

// Config is the main configuration manager.
// It provides thread-safe access to configuration values with zero-allocation reads.
type Config struct {
	engine    *core.Engine
	binder    *bind.Binder
	snapshots *snapshot.Manager
	crypto    *crypto.CryptoManager
	validator *validate.StructValidator
	watcher   *watch.Manager
	sources   []core.Source
	mu        sync.RWMutex
	closed    atomic.Bool
	onChange  []func(context.Context, types.Event)
	hooks     map[types.HookType][]types.Hook
}

// Option configures the Config instance.
type Option func(*Config) error

// New creates a new configuration instance with the given options.
func New(opts ...Option) (*Config, error) {
	c := &Config{
		engine:    core.NewEngine(),
		binder:    bind.NewBinder(),
		snapshots: snapshot.NewManager(100),
		validator: validate.NewStructValidator(),
		watcher:   watch.NewManager(),
		sources:   make([]core.Source, 0),
		onChange:  make([]func(context.Context, types.Event), 0),
		hooks:     make(map[types.HookType][]types.Hook),
	}

	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	// Load initial configuration
	if len(c.sources) > 0 {
		if err := c.reload(context.Background()); err != nil {
			return nil, err
		}
	}

	return c, nil
}

// MustNew creates a new configuration instance, panicking on error.
func MustNew(opts ...Option) *Config {
	c, err := New(opts...)
	if err != nil {
		panic(err)
	}

	return c
}

// WithFile adds a file source.
func WithFile(path string, opts ...source.FileOption) Option {
	return func(c *Config) error {
		resolved, err := source.ResolvePath(path)
		if err != nil {
			return err
		}
		c.sources = append(c.sources, source.NewFileSource(resolved, opts...))

		return nil
	}
}

// WithEnv adds an environment source.
func WithEnv(prefix string, opts ...source.EnvOption) Option {
	return func(c *Config) error {
		opts = append([]source.EnvOption{source.WithEnvPrefix(prefix)}, opts...)
		c.sources = append(c.sources, source.NewEnvSource(opts...))

		return nil
	}
}

// WithMemory adds a memory source.
func WithMemory(data map[string]any, opts ...source.MemoryOption) Option {
	return func(c *Config) error {
		opts = append([]source.MemoryOption{source.WithMemoryData(data)}, opts...)
		c.sources = append(c.sources, source.NewMemorySource(opts...))

		return nil
	}
}

// WithRemote adds a remote source.
func WithRemote(endpoint string, opts ...source.RemoteOption) Option {
	return func(c *Config) error {
		c.sources = append(c.sources, source.NewRemoteSource(endpoint, opts...))

		return nil
	}
}

// WithEncryption enables encryption with the given key.
func WithEncryption(key []byte) Option {
	return func(c *Config) error {
		mgr, err := crypto.NewCryptoManager(key)
		if err != nil {
			return err
		}
		c.crypto = mgr

		return nil
	}
}

// WithObserver adds an observer for configuration changes.
func WithObserver(fn func(context.Context, types.Event)) Option {
	return func(c *Config) error {
		c.onChange = append(c.onChange, fn)

		return nil
	}
}

// WithHook adds a lifecycle hook.
func WithHook(hookType types.HookType, hook types.Hook) Option {
	return func(c *Config) error {
		c.hooks[hookType] = append(c.hooks[hookType], hook)

		return nil
	}
}

// WithWatcher enables file watching for hot reload.
func WithWatcher() Option {
	return func(_ *Config) error {
		return nil // Watcher is already initialized
	}
}

// --- Value Accessors (Lock-Free Reads) ---

// Get retrieves a configuration value by key.
// This operation is O(1) and lock-free.
func (c *Config) Get(key string) (types.Value, bool) {
	state := c.engine.State()

	return state.Get(key)
}

// GetString retrieves a string value by key.
func (c *Config) GetString(key string) string {
	if v, ok := c.Get(key); ok {
		return v.String()
	}

	return ""
}

// GetStringDefault retrieves a string value with a default.
func (c *Config) GetStringDefault(key, def string) string {
	if v, ok := c.Get(key); ok {
		return v.String()
	}

	return def
}

// GetInt retrieves an int value by key.
func (c *Config) GetInt(key string) int {
	if v, ok := c.Get(key); ok {
		if i, okInt := v.Int(); okInt {
			return i
		}
	}

	return 0
}

// GetIntDefault retrieves an int value with a default.
func (c *Config) GetIntDefault(key string, def int) int {
	if v, ok := c.Get(key); ok {
		if i, okInt := v.Int(); okInt {
			return i
		}
	}

	return def
}

// GetInt64 retrieves an int64 value by key.
func (c *Config) GetInt64(key string) int64 {
	if v, ok := c.Get(key); ok {
		if i, okInt := v.Int64(); okInt {
			return i
		}
	}

	return 0
}

// GetFloat64 retrieves a float64 value by key.
func (c *Config) GetFloat64(key string) float64 {
	if v, ok := c.Get(key); ok {
		if f, okFloat := v.Float64(); okFloat {
			return f
		}
	}

	return 0
}

// GetBool retrieves a bool value by key.
func (c *Config) GetBool(key string) bool {
	if v, ok := c.Get(key); ok {
		if b, okBool := v.Bool(); okBool {
			return b
		}
	}

	return false
}

// GetBoolDefault retrieves a bool value with a default.
func (c *Config) GetBoolDefault(key string, def bool) bool {
	if v, ok := c.Get(key); ok {
		if b, okBool := v.Bool(); okBool {
			return b
		}
	}

	return def
}

// GetDuration retrieves a duration value by key.
func (c *Config) GetDuration(key string) time.Duration {
	if v, ok := c.Get(key); ok {
		if d, okDuration := v.Duration(); okDuration {
			return d
		}
	}

	return 0
}

// GetDurationDefault retrieves a duration value with a default.
func (c *Config) GetDurationDefault(key string, def time.Duration) time.Duration {
	if v, ok := c.Get(key); ok {
		if d, okDuration := v.Duration(); okDuration {
			return d
		}
	}

	return def
}

// GetSlice retrieves a slice value by key.
func (c *Config) GetSlice(key string) []any {
	if v, ok := c.Get(key); ok {
		if s, okSlice := v.Raw().([]any); okSlice {
			return s
		}
	}

	return nil
}

// GetMap retrieves a map value by key.
func (c *Config) GetMap(key string) map[string]any {
	if v, ok := c.Get(key); ok {
		if m, okMap := v.Raw().(map[string]any); okMap {
			return m
		}
	}

	return nil
}

// --- Struct Binding ---

// Bind binds configuration values to a struct.
func (c *Config) Bind(target any) error {
	state := c.engine.State()

	return c.binder.Bind(state.GetAll(), target)
}

// MustBind binds configuration values to a struct, panicking on error.
func (c *Config) MustBind(target any) {
	if err := c.Bind(target); err != nil {
		panic(err)
	}
}

// --- Updates ---

// Set sets a configuration value.
func (c *Config) Set(key string, value any) error {
	if c.closed.Load() {
		return types.NewError(types.ErrContextCanceled, "config is closed")
	}

	// Get current state
	state := c.engine.State()
	data := state.GetAll()
	if data == nil {
		data = make(map[string]types.Value)
	}

	// Update value
	data[key] = types.NewValue(value, inferType(value), types.SourceMemory, 30)

	// Create new state
	newState := core.NewState(data)

	// Atomic update
	return c.engine.Update(context.Background(), newState)
}

// Delete removes a configuration value.
func (c *Config) Delete(key string) error {
	if c.closed.Load() {
		return types.NewError(types.ErrContextCanceled, "config is closed")
	}

	state := c.engine.State()
	data := state.GetAll()

	delete(data, key)

	newState := core.NewState(data)

	return c.engine.Update(context.Background(), newState)
}

// --- Reloading ---

// Reload reloads configuration from all sources.
func (c *Config) Reload(ctx context.Context) error {
	return c.reload(ctx)
}

func (c *Config) reload(ctx context.Context) error {
	c.mu.RLock()
	sources := make([]core.Source, len(c.sources))
	copy(sources, c.sources)
	c.mu.RUnlock()

	return c.engine.Load(ctx, sources)
}

// --- Hot Reload ---

// Watch starts watching for configuration changes.
func (c *Config) Watch(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Start watching file sources
	for i, s := range c.sources {
		if fs, ok := s.(*source.FileSource); ok {
			// Capture index for closure
			idx := i
			err := fs.Watch(ctx, func() {
				// Reload when file changes
				_ = c.Reload(ctx)

				// Notify observers
				c.notifyObservers(ctx, types.Event{
					Type:      types.EventReload,
					Timestamp: time.Now(),
					Source:    types.SourceFile,
				})
			})
			if err != nil {
				return err
			}
			c.sources[idx] = fs
		}
	}

	return nil
}

// --- Observers ---

// Observe registers an observer for configuration changes.
func (c *Config) Observe(fn func(context.Context, types.Event)) (func(), error) {
	return c.engine.Observe(context.Background(), func(ctx context.Context, event types.Event) error {
		fn(ctx, event)

		return nil
	})
}

func (c *Config) notifyObservers(ctx context.Context, event types.Event) {
	for _, fn := range c.onChange {
		go fn(ctx, event)
	}
}

// --- Snapshots ---

// Snapshot creates a configuration snapshot.
func (c *Config) Snapshot(opts ...snapshot.SnapshotOption) *snapshot.Snapshot {
	state := c.engine.State()

	return c.snapshots.Take(state.GetAll(), opts...)
}

// Restore restores configuration from a snapshot.
func (c *Config) Restore(s *snapshot.Snapshot) error {
	if s == nil {
		return types.NewError(types.ErrNotFound, "snapshot is nil")
	}

	newState := core.NewState(s.Data())

	return c.engine.Update(context.Background(), newState)
}

// LatestSnapshot returns the most recent snapshot.
func (c *Config) LatestSnapshot() *snapshot.Snapshot {
	return c.snapshots.Latest()
}

// Snapshots returns all snapshots.
func (c *Config) Snapshots() []*snapshot.Snapshot {
	return c.snapshots.List(time.Time{}, time.Time{})
}

// --- Export ---

// Export exports configuration to the specified format.
func (c *Config) Export(w io.Writer, format export.Format) error {
	state := c.engine.State()
	registry := export.NewRegistry()

	return registry.Export(state.GetAll(), w, format)
}

// ExportJSON exports configuration to JSON format.
func (c *Config) ExportJSON() ([]byte, error) {
	state := c.engine.State()

	return export.ToJSON(state.GetAll())
}

// ExportYAML exports configuration to YAML format.
func (c *Config) ExportYAML() ([]byte, error) {
	state := c.engine.State()

	return export.ToYAML(state.GetAll())
}

// ExportTOML exports configuration to TOML format.
func (c *Config) ExportTOML() ([]byte, error) {
	state := c.engine.State()

	return export.ToTOML(state.GetAll())
}

// ExportEnv exports configuration to environment variable format.
func (c *Config) ExportEnv() ([]byte, error) {
	state := c.engine.State()

	return export.ToEnv(state.GetAll())
}

// --- Validation ---

// Validate validates the configuration against a validation plan.
func (c *Config) Validate(plan *validate.Plan) error {
	state := c.engine.State()

	return plan.Validate(context.Background(), state.GetAll())
}

// ValidateStruct validates a struct using struct tags.
func (c *Config) ValidateStruct(target any) error {
	return c.validator.Validate(context.Background(), target)
}

// --- Encryption ---

// Encrypt encrypts a value.
func (c *Config) Encrypt(plaintext string) (*crypto.EncryptedValue, error) {
	if c.crypto == nil {
		return nil, types.NewError(types.ErrCryptoError, "encryption not enabled")
	}

	return c.crypto.EncryptString(plaintext)
}

// Decrypt decrypts a value.
func (c *Config) Decrypt(ev *crypto.EncryptedValue) (string, error) {
	if c.crypto == nil {
		return "", types.NewError(types.ErrCryptoError, "encryption not enabled")
	}

	return c.crypto.DecryptString(ev)
}

// --- Schema ---

// Schema generates a schema from a struct type.
func (c *Config) Schema(target any) (*bind.Schema, error) {
	return c.binder.Schema(target)
}

// --- Keys ---

// Keys returns all configuration keys.
func (c *Config) Keys() []string {
	state := c.engine.State()

	return state.Keys()
}

// Has checks if a key exists.
func (c *Config) Has(key string) bool {
	_, ok := c.Get(key)

	return ok
}

// --- Lifecycle ---

// Close closes the configuration and releases resources.
func (c *Config) Close(ctx context.Context) error {
	if c.closed.Swap(true) {
		return nil
	}

	// Close all sources
	c.mu.RLock()
	sources := c.sources
	c.mu.RUnlock()

	for _, s := range sources {
		_ = s.Close(ctx)
	}

	// Close watcher
	if c.watcher != nil {
		_ = c.watcher.Stop()
	}

	// Close engine
	return c.engine.Close(ctx)
}

// Done returns a channel that's closed when the config is closed.
func (c *Config) Done() <-chan struct{} {
	return c.engine.Done()
}

// --- Utility Functions ---

// inferType infers the value type from the raw value.
func inferType(v any) types.ValueType {
	switch v.(type) {
	case string:
		return types.TypeString
	case int:
		return types.TypeInt
	case int64:
		return types.TypeInt64
	case float64:
		return types.TypeFloat64
	case bool:
		return types.TypeBool
	case time.Duration:
		return types.TypeDuration
	case time.Time:
		return types.TypeTime
	case []any:
		return types.TypeSlice
	case map[string]any:
		return types.TypeMap
	default:
		return types.TypeUnknown
	}
}

// LoadFile loads configuration from a file.
func LoadFile(path string, opts ...Option) (*Config, error) {
	return New(append([]Option{WithFile(path)}, opts...)...)
}

// LoadEnv loads configuration from environment variables.
func LoadEnv(prefix string, opts ...Option) (*Config, error) {
	return New(append([]Option{WithEnv(prefix)}, opts...)...)
}

// Load loads configuration from multiple sources.
func Load(sources ...Option) (*Config, error) {
	return New(sources...)
}

// --- Debug ---

// Debug returns debug information about the configuration.
func (c *Config) Debug() map[string]any {
	state := c.engine.State()

	return map[string]any{
		"keys":       c.Keys(),
		"version":    state.Version(),
		"createdAt":  state.CreatedAt(),
		"snapshots":  c.snapshots.Count(),
		"encryption": c.crypto != nil,
		"closed":     c.closed.Load(),
	}
}

// String returns a string representation of the configuration.
func (c *Config) String() string {
	data, err := c.ExportJSON()
	if err != nil {
		return "{}"
	}

	return string(data)
}

// MarshalJSON implements JSON marshaling.
func (c *Config) MarshalJSON() ([]byte, error) {
	return c.ExportJSON()
}

// UnmarshalJSON is not supported for Config.
func (c *Config) UnmarshalJSON(_ []byte) error {
	return types.NewError(types.ErrInvalidFormat, "cannot unmarshal into Config")
}

// --- Global Instance (Optional Convenience) ---

var (
	globalConfig atomic.Pointer[Config]
	globalMu     sync.Mutex
)

// Init initializes the global configuration.
func Init(opts ...Option) error {
	globalMu.Lock()
	defer globalMu.Unlock()

	c, err := New(opts...)
	if err != nil {
		return err
	}

	globalConfig.Store(c)

	return nil
}

// Global returns the global configuration instance.
func Global() *Config {
	return globalConfig.Load()
}

// CloseGlobal closes the global configuration.
func CloseGlobal(ctx context.Context) error {
	globalMu.Lock()
	defer globalMu.Unlock()

	c := globalConfig.Load()
	if c == nil {
		return nil
	}

	globalConfig.Store(nil)

	return c.Close(ctx)
}

// --- Backward Compatibility ---

// All returns all configuration data.
func (c *Config) All() map[string]types.Value {
	state := c.engine.State()

	return state.GetAll()
}

// Data returns the raw configuration data.
func (c *Config) Data() map[string]any {
	state := c.engine.State()
	data := state.GetAll()

	result := make(map[string]any, len(data))
	for k, v := range data {
		result[k] = v.Raw()
	}

	return result
}

// --- Context Helpers ---

type configKey struct{}

// ContextWithConfig returns a new context with the config attached.
func ContextWithConfig(ctx context.Context, c *Config) context.Context {
	return context.WithValue(ctx, configKey{}, c)
}

// ConfigFromContext retrieves the config from a context.
func ConfigFromContext(ctx context.Context) *Config {
	if c, ok := ctx.Value(configKey{}).(*Config); ok {
		return c
	}

	return nil
}

// --- File Helpers ---

// ReadFile reads a configuration file.
func ReadFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	format := parser.DetectFormat(path)
	registry := parser.NewRegistry()
	p, err := registry.Get(format)
	if err != nil {
		return nil, err
	}

	return p.Parse(data)
}

// WriteFile writes configuration to a file.
func WriteFile(path string, data map[string]types.Value, format export.Format) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	registry := export.NewRegistry()

	return registry.Export(data, f, format)
}

// Print prints the configuration to stdout.
func (c *Config) Print() error {
	return c.Export(os.Stdout, export.FormatJSON)
}

// PrintYAML prints the configuration as YAML to stdout.
func (c *Config) PrintYAML() error {
	return c.Export(os.Stdout, export.FormatYAML)
}

// --- Merge Helpers ---

// Merge merges another configuration into this one.
func (c *Config) Merge(other *Config) error {
	if other == nil {
		return nil
	}

	state := c.engine.State()
	otherState := other.engine.State()

	merger := merge.NewMerger(merge.StrategyLast)
	merged := merger.Merge(state.GetAll(), otherState.GetAll())

	newState := core.NewState(merged)

	return c.engine.Update(context.Background(), newState)
}

// --- Clone ---

// Clone creates a copy of the configuration.
func (c *Config) Clone() *Config {
	state := c.engine.State()

	clone, _ := New(
		WithMemory(state.Export()),
	)

	return clone
}

// --- Stats ---

// Stats returns configuration statistics.
func (c *Config) Stats() Stats {
	state := c.engine.State()

	return Stats{
		KeyCount:      len(state.Keys()),
		Version:       state.Version(),
		SnapshotCount: c.snapshots.Count(),
		HasEncryption: c.crypto != nil,
		IsClosed:      c.closed.Load(),
	}
}

// Stats holds configuration statistics.
type Stats struct {
	KeyCount      int
	Version       uint64
	SnapshotCount int
	HasEncryption bool
	IsClosed      bool
}

// --- JSON Support ---

// ToJSON returns the configuration as JSON.
func (c *Config) ToJSON() (string, error) {
	data, err := c.ExportJSON()
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// ToYAML returns the configuration as YAML.
func (c *Config) ToYAML() (string, error) {
	data, err := c.ExportYAML()
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// --- Template Processing ---

// ProcessTemplates processes template placeholders in configuration values.
// Placeholders are in the format ${key} or ${key:default}.
func (c *Config) ProcessTemplates() error {
	state := c.engine.State()
	data := state.GetAll()

	changed := false

	for key, value := range data {
		if s, ok := value.Raw().(string); ok {
			processed := c.processTemplate(s)
			if processed != s {
				data[key] = types.NewValue(processed, types.TypeString, value.Source(), value.Priority())
				changed = true
			}
		}
	}

	if changed {
		newState := core.NewState(data)

		return c.engine.Update(context.Background(), newState)
	}

	return nil
}

func (c *Config) processTemplate(s string) string {
	return c.processTemplateDepth(s, 0)
}

const maxTemplateDepth = 16

func (c *Config) processTemplateDepth(s string, depth int) string {
	if depth >= maxTemplateDepth {
		return s
	}

	result := s

	for i := 0; i < len(result); i++ {
		if result[i] != '$' || i+1 >= len(result) || result[i+1] != '{' {
			continue
		}

		// Find closing brace
		end := -1
		for j := i + 2; j < len(result); j++ {
			if result[j] == '}' {
				end = j

				break
			}
		}

		if end == -1 {
			continue
		}

		// Extract placeholder
		placeholder := result[i+2 : end]

		// Parse key and default
		var key, def string
		if colon := findColon(placeholder); colon >= 0 {
			key = placeholder[:colon]
			def = placeholder[colon+1:]
		} else {
			key = placeholder
		}

		// Get value
		var replacement string
		if v, ok := c.Get(key); ok {
			replacement = c.processTemplateDepth(v.String(), depth+1)
		} else if def != "" {
			replacement = def
		} else {
			// Keep placeholder if no value and no default
			continue
		}

		// Replace placeholder
		result = result[:i] + replacement + result[end+1:]
		i += len(replacement) - 1
	}

	return result
}

func findColon(s string) int {
	for i := range len(s) {
		if s[i] == ':' {
			return i
		}
	}

	return -1
}

// --- JSON Marshal/Unmarshal for External Use ---

// JSON returns a JSON representation suitable for APIs.
func (c *Config) JSON() string {
	data, err := c.ExportJSON()
	if err != nil {
		return "{}"
	}

	return string(data)
}

// PrettyJSON returns a pretty-printed JSON representation.
func (c *Config) PrettyJSON() string {
	data, err := c.ExportJSON()
	if err != nil {
		return "{}"
	}

	buf := make([]byte, 0, len(data))
	buf = append(buf, data...)

	// Already pretty printed by our exporter
	return string(buf)
}

// --- Map Functions ---

// AsMap returns configuration as a map.
func (c *Config) AsMap() map[string]any {
	return c.Data()
}

// AsFlatMap returns configuration as a flattened map.
func (c *Config) AsFlatMap() map[string]string {
	state := c.engine.State()
	data := state.GetAll()

	result := make(map[string]string, len(data))
	for k, v := range data {
		result[k] = v.String()
	}

	return result
}

// --- Introspection ---

// Type returns the type of a configuration value.
func (c *Config) Type(key string) types.ValueType {
	if v, ok := c.Get(key); ok {
		return v.Type()
	}

	return types.TypeUnknown
}

// Source returns the source of a configuration value.
func (c *Config) Source(key string) types.SourceType {
	if v, ok := c.Get(key); ok {
		return v.Source()
	}

	return types.SourceNone
}
