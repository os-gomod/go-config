// Package loader provides configuration loading utilities with optimized memory management.
// Uses sync.Pool for reusable objects to minimize allocations.
package loader

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/os-gomod/go-config/core"
	"github.com/os-gomod/go-config/types"
)

// Loader loads configuration from multiple sources.
type Loader struct {
	sources []core.Source
	mu      sync.RWMutex
}

// New creates a new loader.
func New() *Loader {
	return &Loader{
		sources: make([]core.Source, 0),
	}
}

// Add adds a source to the loader.
func (l *Loader) Add(source core.Source) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sources = append(l.sources, source)
}

// Load loads configuration from all sources.
func (l *Loader) Load(ctx context.Context) (map[string]types.Value, error) {
	l.mu.RLock()
	sources := make([]core.Source, len(l.sources))
	copy(sources, l.sources)
	l.mu.RUnlock()

	// Get buffer from pool
	result := getValueMap()
	defer putValueMap(result)

	for _, source := range sources {
		data, err := source.Load(ctx)
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

	// Create result map (caller's responsibility)
	final := make(map[string]types.Value, len(result))
	for k, v := range result {
		final[k] = v
	}

	return final, nil
}

// Close closes all sources.
func (l *Loader) Close(ctx context.Context) error {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var lastErr error
	for _, source := range l.sources {
		if err := source.Close(ctx); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// --- sync.Pool for reusable objects ---

var (
	// Pool for map[string]types.Value.
	valueMapPool = sync.Pool{
		New: func() any {
			return make(map[string]types.Value, 64)
		},
	}

	// Pool for []byte.
	byteSlicePool = sync.Pool{
		New: func() any {
			b := make([]byte, 0, 4096)

			return &b
		},
	}

	// Pool for strings.Builder.
	stringBuilderPool = sync.Pool{
		New: func() any {
			return new(strings.Builder)
		},
	}
)

// getValueMap gets a map from the pool.
func getValueMap() map[string]types.Value {
	return valueMapPool.Get().(map[string]types.Value)
}

// putValueMap returns a map to the pool.
func putValueMap(m map[string]types.Value) {
	// Clear the map
	for k := range m {
		delete(m, k)
	}
	valueMapPool.Put(m)
}

// GetByteSlice gets a byte slice from the pool.
func GetByteSlice() *[]byte {
	return byteSlicePool.Get().(*[]byte)
}

// PutByteSlice returns a byte slice to the pool.
func PutByteSlice(b *[]byte) {
	*b = (*b)[:0]
	byteSlicePool.Put(b)
}

// GetStringBuilder gets a strings.Builder from the pool.
func GetStringBuilder() *strings.Builder {
	sb := stringBuilderPool.Get().(*strings.Builder)
	sb.Reset()

	return sb
}

// PutStringBuilder returns a strings.Builder to the pool.
func PutStringBuilder(sb *strings.Builder) {
	sb.Reset()
	stringBuilderPool.Put(sb)
}

// --- Buffered Loader ---

// BufferedLoader loads configuration with buffered I/O.
type BufferedLoader struct {
	*Loader
	bufferSize int
}

// NewBuffered creates a new buffered loader.
func NewBuffered(bufferSize int) *BufferedLoader {
	return &BufferedLoader{
		Loader:     New(),
		bufferSize: bufferSize,
	}
}

// --- Lazy Loader ---

// LazyLoader loads configuration on first access.
type LazyLoader struct {
	loader   *Loader
	data     map[string]types.Value
	loaded   bool
	loadOnce sync.Once
	loadErr  error
	mu       sync.RWMutex
}

// NewLazy creates a new lazy loader.
func NewLazy() *LazyLoader {
	return &LazyLoader{
		loader: New(),
		data:   make(map[string]types.Value),
	}
}

// Add adds a source.
func (l *LazyLoader) Add(source core.Source) {
	l.loader.Add(source)
}

// Load loads configuration on first call.
func (l *LazyLoader) Load(ctx context.Context) (map[string]types.Value, error) {
	l.loadOnce.Do(func() {
		data, err := l.loader.Load(ctx)
		if err != nil {
			l.loadErr = err

			return
		}

		l.mu.Lock()
		l.data = data
		l.loaded = true
		l.mu.Unlock()
	})

	if l.loadErr != nil {
		return nil, l.loadErr
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	// Return a copy
	result := make(map[string]types.Value, len(l.data))
	for k, v := range l.data {
		result[k] = v
	}

	return result, nil
}

// IsLoaded returns true if configuration has been loaded.
func (l *LazyLoader) IsLoaded() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.loaded
}

// Reload forces a reload of configuration.
func (l *LazyLoader) Reload(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := l.loader.Load(ctx)
	if err != nil {
		return err
	}

	l.data = data
	l.loaded = true

	return nil
}

// --- Concurrent Loader ---

// ConcurrentLoader loads configuration from sources concurrently.
type ConcurrentLoader struct {
	sources []core.Source
	workers int
	mu      sync.RWMutex
}

// NewConcurrent creates a new concurrent loader.
func NewConcurrent(workers int) *ConcurrentLoader {
	if workers <= 0 {
		workers = 4
	}

	return &ConcurrentLoader{
		sources: make([]core.Source, 0),
		workers: workers,
	}
}

// Add adds a source.
func (l *ConcurrentLoader) Add(source core.Source) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sources = append(l.sources, source)
}

// Load loads configuration concurrently.
func (l *ConcurrentLoader) Load(ctx context.Context) (map[string]types.Value, error) {
	l.mu.RLock()
	sources := make([]core.Source, len(l.sources))
	copy(sources, l.sources)
	l.mu.RUnlock()

	if len(sources) == 0 {
		return make(map[string]types.Value), nil
	}

	type result struct {
		data map[string]types.Value
		err  error
	}

	results := make(chan result, len(sources))

	// Load each source in a goroutine
	for _, source := range sources {
		go func(s core.Source) {
			data, err := s.Load(ctx)
			results <- result{data: data, err: err}
		}(source)
	}

	// Collect results
	final := make(map[string]types.Value)
	for range sources {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case r := <-results:
			if r.err != nil {
				return nil, r.err
			}
			for k, v := range r.data {
				if existing, ok := final[k]; !ok || v.Priority() > existing.Priority() {
					final[k] = v
				}
			}
		}
	}

	return final, nil
}

// Close closes all sources.
func (l *ConcurrentLoader) Close(ctx context.Context) error {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var lastErr error
	for _, source := range l.sources {
		if err := source.Close(ctx); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// --- Chained Loader ---

// ChainedLoader loads configuration in a chain.
type ChainedLoader struct {
	loaders []core.Source
}

// NewChained creates a new chained loader.
func NewChained() *ChainedLoader {
	return &ChainedLoader{
		loaders: make([]core.Source, 0),
	}
}

// Add adds a loader to the chain.
func (l *ChainedLoader) Add(loader core.Source) {
	l.loaders = append(l.loaders, loader)
}

// Load loads configuration in order.
func (l *ChainedLoader) Load(ctx context.Context) (map[string]types.Value, error) {
	result := make(map[string]types.Value)

	for _, loader := range l.loaders {
		data, err := loader.Load(ctx)
		if err != nil {
			return nil, err
		}

		// Merge
		for k, v := range data {
			result[k] = v
		}
	}

	return result, nil
}

// --- Cached Loader ---

// CachedLoader caches loaded configuration.
type CachedLoader struct {
	loader    core.Source
	cache     map[string]types.Value
	cacheTime time.Time
	ttl       time.Duration
	mu        sync.RWMutex
}

// NewCached creates a new cached loader.
func NewCached(loader core.Source, ttl time.Duration) *CachedLoader {
	return &CachedLoader{
		loader: loader,
		ttl:    ttl,
		cache:  make(map[string]types.Value),
	}
}

// Load loads configuration with caching.
func (l *CachedLoader) Load(ctx context.Context) (map[string]types.Value, error) {
	l.mu.RLock()
	if l.cacheTime.Add(l.ttl).After(time.Now()) && len(l.cache) > 0 {
		result := make(map[string]types.Value, len(l.cache))
		for k, v := range l.cache {
			result[k] = v
		}
		l.mu.RUnlock()

		return result, nil
	}
	l.mu.RUnlock()

	data, err := l.loader.Load(ctx)
	if err != nil {
		return nil, err
	}

	l.mu.Lock()
	l.cache = data
	l.cacheTime = time.Now()
	l.mu.Unlock()

	// Return a copy
	result := make(map[string]types.Value, len(data))
	for k, v := range data {
		result[k] = v
	}

	return result, nil
}

// Invalidate invalidates the cache.
func (l *CachedLoader) Invalidate() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cacheTime = time.Time{}
}

// Close closes the underlying loader.
func (l *CachedLoader) Close(ctx context.Context) error {
	return l.loader.Close(ctx)
}
