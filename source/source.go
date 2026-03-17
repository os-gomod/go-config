// Package source provides configuration source implementations.
// Supports file, environment, memory, and remote sources.
package source

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/os-gomod/go-config/confparser"
	"github.com/os-gomod/go-config/types"
)

// Base provides common functionality for sources.
type Base struct {
	name       string
	priority   int
	sourceType types.SourceType
}

// Name returns the source name.
func (b *Base) Name() string { return b.name }

// Priority returns the source priority.
func (b *Base) Priority() int { return b.priority }

// Type returns the source type.
func (b *Base) Type() types.SourceType { return b.sourceType }

// FileSource loads configuration from a file.
type FileSource struct {
	Base
	path    string
	format  confparser.Format
	watcher *FileWatcher
	watched atomic.Bool

	mu sync.RWMutex
}

// FileOption configures a file source.
type FileOption func(*FileSource)

// WithFilePriority sets the source priority.
func WithFilePriority(p int) FileOption {
	return func(f *FileSource) { f.priority = p }
}

// WithFileFormat sets the file format explicitly.
func WithFileFormat(format confparser.Format) FileOption {
	return func(f *FileSource) { f.format = format }
}

// NewFileSource creates a new file source.
func NewFileSource(path string, opts ...FileOption) *FileSource {
	f := &FileSource{
		Base: Base{
			name:       "file:" + path,
			priority:   10,
			sourceType: types.SourceFile,
		},
		path:   path,
		format: confparser.FormatAuto,
	}

	for _, opt := range opts {
		opt(f)
	}

	// Auto-detect format if needed
	if f.format == confparser.FormatAuto {
		f.format = confparser.DetectFormat(path)
	}

	return f
}

// Load reads configuration from the file.
func (f *FileSource) Load(_ context.Context) (map[string]types.Value, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	data, err := os.ReadFile(f.path)
	if err != nil {
		return nil, types.NewError(types.ErrSourceError, "failed to read file",
			types.WithSource(types.SourceFile),
			types.WithCause(err))
	}

	// Get parser
	registry := confparser.NewRegistry()
	p, err := registry.Get(f.format)
	if err != nil {
		return nil, err
	}

	// Parse file
	parsed, err := p.Parse(data)
	if err != nil {
		return nil, err
	}

	// Flatten nested structure
	flattened := confparser.Flatten(parsed, "")

	// Convert to values
	result := make(map[string]types.Value, len(flattened))
	for k, v := range flattened {
		result[k] = types.NewValue(
			v,
			inferType(v),
			types.SourceFile,
			f.priority,
		)
	}

	return result, nil
}

// Watch starts watching the file for changes.
func (f *FileSource) Watch(ctx context.Context, onChange func()) error {
	if f.watched.Swap(true) {
		return nil // Already watching
	}

	f.watcher = NewFileWatcher(f.path)

	return f.watcher.Start(ctx, onChange)
}

// Close releases resources.
func (f *FileSource) Close(_ context.Context) error {
	if f.watcher != nil {
		return f.watcher.Stop()
	}

	return nil
}

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

// EnvSource loads configuration from environment variables.
type EnvSource struct {
	Base
	prefix    string
	transform func(string) string
}

// EnvOption configures an environment source.
type EnvOption func(*EnvSource)

// WithEnvPrefix sets the environment variable prefix.
func WithEnvPrefix(prefix string) EnvOption {
	return func(e *EnvSource) { e.prefix = prefix }
}

// WithEnvPriority sets the source priority.
func WithEnvPriority(p int) EnvOption {
	return func(e *EnvSource) { e.priority = p }
}

// WithEnvTransform sets a key transformation function.
func WithEnvTransform(fn func(string) string) EnvOption {
	return func(e *EnvSource) { e.transform = fn }
}

// NewEnvSource creates a new environment source.
func NewEnvSource(opts ...EnvOption) *EnvSource {
	e := &EnvSource{
		Base: Base{
			name:       "env",
			priority:   20, // Higher priority than files
			sourceType: types.SourceEnv,
		},
		transform: defaultEnvTransform,
	}

	for _, opt := range opts {
		opt(e)
	}

	return e
}

// Load reads configuration from environment variables.
func (e *EnvSource) Load(_ context.Context) (map[string]types.Value, error) {
	result := make(map[string]types.Value)

	environ := os.Environ()
	for _, env := range environ {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		// Filter by prefix
		if e.prefix != "" {
			if !strings.HasPrefix(key, e.prefix) {
				continue
			}
			key = strings.TrimPrefix(key, e.prefix)
		}

		// Transform key
		configKey := e.transform(key)

		// Parse value
		result[configKey] = types.NewValue(
			parseEnvValue(value),
			inferType(parseEnvValue(value)),
			types.SourceEnv,
			e.priority,
		)
	}

	return result, nil
}

// Close is a no-op for environment source.
func (e *EnvSource) Close(_ context.Context) error {
	return nil
}

// defaultEnvTransform converts ENV_VAR to env.var format.
func defaultEnvTransform(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, "_", "."))
}

// parseEnvValue parses an environment variable value.
func parseEnvValue(s string) any {
	// Boolean
	switch strings.ToLower(s) {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	}

	// Integer
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}

	// Float
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}

	// Duration
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}

	// String
	return s
}

// MemorySource provides in-memory configuration storage.
type MemorySource struct {
	Base
	data map[string]any
	mu   sync.RWMutex
}

// MemoryOption configures a memory source.
type MemoryOption func(*MemorySource)

// WithMemoryPriority sets the source priority.
func WithMemoryPriority(p int) MemoryOption {
	return func(m *MemorySource) { m.priority = p }
}

// WithMemoryData sets initial data.
func WithMemoryData(data map[string]any) MemoryOption {
	return func(m *MemorySource) { m.data = data }
}

// NewMemorySource creates a new memory source.
func NewMemorySource(opts ...MemoryOption) *MemorySource {
	m := &MemorySource{
		Base: Base{
			name:       "memory",
			priority:   30, // Highest priority
			sourceType: types.SourceMemory,
		},
		data: make(map[string]any),
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// Load returns the current memory configuration.
func (m *MemorySource) Load(_ context.Context) (map[string]types.Value, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	flattened := confparser.Flatten(m.data, "")
	result := make(map[string]types.Value, len(flattened))
	for k, v := range flattened {
		result[k] = types.NewValue(
			v,
			inferType(v),
			types.SourceMemory,
			m.priority,
		)
	}

	return result, nil
}

// Set sets a value in memory.
func (m *MemorySource) Set(key string, value any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
}

// Delete removes a value from memory.
func (m *MemorySource) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
}

// Close is a no-op for memory source.
func (m *MemorySource) Close(_ context.Context) error {
	return nil
}

// RemoteSource loads configuration from a remote endpoint.
type RemoteSource struct {
	Base
	endpoint    string
	fetcher     RemoteFetcher
	cache       map[string]types.Value
	cacheExpiry time.Time
	ttl         time.Duration
	mu          sync.RWMutex
}

// RemoteFetcher defines the interface for remote configuration fetching.
type RemoteFetcher interface {
	Fetch(ctx context.Context, endpoint string) (map[string]any, error)
}

// RemoteOption configures a remote source.
type RemoteOption func(*RemoteSource)

// WithRemotePriority sets the source priority.
func WithRemotePriority(p int) RemoteOption {
	return func(r *RemoteSource) { r.priority = p }
}

// WithRemoteTTL sets the cache TTL.
func WithRemoteTTL(ttl time.Duration) RemoteOption {
	return func(r *RemoteSource) { r.ttl = ttl }
}

// WithRemoteFetcher sets the fetcher implementation.
func WithRemoteFetcher(f RemoteFetcher) RemoteOption {
	return func(r *RemoteSource) { r.fetcher = f }
}

// NewRemoteSource creates a new remote source.
func NewRemoteSource(endpoint string, opts ...RemoteOption) *RemoteSource {
	r := &RemoteSource{
		Base: Base{
			name:       "remote:" + endpoint,
			priority:   15,
			sourceType: types.SourceRemote,
		},
		endpoint: endpoint,
		ttl:      5 * time.Minute,
		cache:    make(map[string]types.Value),
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// Load fetches configuration from the remote endpoint.
func (r *RemoteSource) Load(ctx context.Context) (map[string]types.Value, error) {
	r.mu.RLock()

	// Check cache
	if r.cacheExpiry.After(time.Now()) && len(r.cache) > 0 {
		result := r.cache
		r.mu.RUnlock()

		return result, nil
	}

	r.mu.RUnlock()

	// Fetch from remote
	if r.fetcher == nil {
		return nil, types.NewError(types.ErrSourceError, "no fetcher configured")
	}

	data, err := r.fetcher.Fetch(ctx, r.endpoint)
	if err != nil {
		return nil, types.NewError(types.ErrSourceError, "failed to fetch remote config",
			types.WithSource(types.SourceRemote),
			types.WithCause(err))
	}

	// Flatten and convert
	flattened := confparser.Flatten(data, "")
	result := make(map[string]types.Value, len(flattened))
	for k, v := range flattened {
		result[k] = types.NewValue(
			v,
			inferType(v),
			types.SourceRemote,
			r.priority,
		)
	}

	// Update cache
	r.mu.Lock()
	r.cache = result
	r.cacheExpiry = time.Now().Add(r.ttl)
	r.mu.Unlock()

	return result, nil
}

// InvalidateCache clears the cache.
func (r *RemoteSource) InvalidateCache() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cacheExpiry = time.Time{}
}

// Close is a no-op for remote source.
func (r *RemoteSource) Close(_ context.Context) error {
	return nil
}

// FileWatcher watches a file for changes.
type FileWatcher struct {
	path     string
	modTime  time.Time
	interval time.Duration
	stop     chan struct{}
	stopped  chan struct{}
	mu       sync.Mutex
}

// NewFileWatcher creates a new file watcher.
func NewFileWatcher(path string) *FileWatcher {
	return &FileWatcher{
		path:     path,
		interval: time.Second,
		stop:     make(chan struct{}),
		stopped:  make(chan struct{}),
	}
}

// Start begins watching the file.
func (w *FileWatcher) Start(ctx context.Context, onChange func()) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Get initial mod time
	info, err := os.Stat(w.path)
	if err != nil {
		return err
	}
	w.modTime = info.ModTime()

	go w.watch(ctx, onChange)

	return nil
}

// watch polls the file for changes.
func (w *FileWatcher) watch(ctx context.Context, onChange func()) {
	defer close(w.stopped)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stop:
			return
		case <-ticker.C:
			info, err := os.Stat(w.path)
			if err != nil {
				continue
			}

			if info.ModTime().After(w.modTime) {
				w.modTime = info.ModTime()
				onChange()
			}
		}
	}
}

// Stop stops watching the file.
func (w *FileWatcher) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	select {
	case <-w.stopped:
		return nil
	default:
		close(w.stop)
		<-w.stopped
	}

	return nil
}

// MultiSource combines multiple sources.
type MultiSource struct {
	sources []Source
}

// Source is the base interface for all sources.
type Source interface {
	Load(ctx context.Context) (map[string]types.Value, error)
	Close(ctx context.Context) error
}

// NewMultiSource creates a new multi-source.
func NewMultiSource(sources ...Source) *MultiSource {
	return &MultiSource{sources: sources}
}

// Add adds a source to the multi-source.
func (m *MultiSource) Add(s Source) {
	m.sources = append(m.sources, s)
}

// Load loads from all sources and merges results.
func (m *MultiSource) Load(ctx context.Context) (map[string]types.Value, error) {
	result := make(map[string]types.Value)

	for _, s := range m.sources {
		data, err := s.Load(ctx)
		if err != nil {
			return nil, err
		}

		// Merge with priority handling
		for k, v := range data {
			if existing, ok := result[k]; !ok || v.Priority() > existing.Priority() {
				result[k] = v
			}
		}
	}

	return result, nil
}

// Close closes all sources.
func (m *MultiSource) Close(ctx context.Context) error {
	var lastErr error
	for _, s := range m.sources {
		if err := s.Close(ctx); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// ResolvePath resolves a configuration file path.
func ResolvePath(path string) (string, error) {
	// Handle absolute path
	if filepath.IsAbs(path) {
		return path, nil
	}

	// Check current directory
	if _, err := os.Stat(path); err == nil {
		absPath, absErr := filepath.Abs(path)
		if absErr != nil {
			return "", absErr
		}

		return absPath, nil
	}

	// Check common locations
	locations := []string{
		"./config",
		"./configs",
		"/etc",
	}

	for _, loc := range locations {
		fullPath := filepath.Join(loc, path)
		if _, err := os.Stat(fullPath); err == nil {
			absPath, absErr := filepath.Abs(fullPath)
			if absErr != nil {
				return "", absErr
			}

			return absPath, nil
		}
	}

	return "", types.NewError(types.ErrSourceError, "configuration file not found: "+path)
}
