// Package watch provides hot reload functionality with goroutine lifecycle control.
// Implements file watching and manual reload with zero goroutine leaks.
package watch

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/os-gomod/go-config/types"
)

// Watcher watches for configuration changes.
type Watcher interface {
	// Start begins watching for changes.
	Start(ctx context.Context) error

	// Stop stops watching.
	Stop() error

	// Changes returns a channel for change notifications.
	Changes() <-chan Change
}

// Change represents a configuration change.
type Change struct {
	Type      ChangeType
	Source    string
	Timestamp time.Time
	Error     error
}

// ChangeType represents the type of change.
type ChangeType uint8

const (
	ChangeModified ChangeType = iota
	ChangeCreated
	ChangeDeleted
	ChangeReload
)

// Manager manages multiple watchers.
type Manager struct {
	watchers  map[string]Watcher
	callbacks []Callback
	running   atomic.Bool
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// Callback is called when configuration changes.
type Callback func(ctx context.Context, change Change) error

// NewManager creates a new watch manager.
func NewManager() *Manager {
	return &Manager{
		watchers:  make(map[string]Watcher),
		callbacks: make([]Callback, 0),
	}
}

// Register adds a watcher with a unique ID.
func (m *Manager) Register(id string, w Watcher) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.watchers[id] = w
}

// Subscribe adds a change callback.
func (m *Manager) Subscribe(cb Callback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callbacks = append(m.callbacks, cb)
}

// Start starts all watchers.
func (m *Manager) Start(ctx context.Context) error {
	if m.running.Swap(true) {
		return nil // Already running
	}

	runCtx, cancel := context.WithCancel(ctx)
	m.ctx, m.cancel = runCtx, cancel

	m.mu.RLock()
	defer m.mu.RUnlock()

	for id, w := range m.watchers {
		if err := w.Start(runCtx); err != nil {
			return types.NewError(types.ErrWatchError, "failed to start watcher: "+id,
				types.WithCause(err))
		}

		// Start goroutine to forward changes
		m.wg.Add(1)
		go m.forwardChanges(id, w)
	}

	return nil
}

// forwardChanges forwards changes from a watcher to callbacks.
func (m *Manager) forwardChanges(id string, w Watcher) {
	defer m.wg.Done()

	for {
		select {
		case <-m.ctx.Done():
			return
		case change, ok := <-w.Changes():
			if !ok {
				return
			}

			change.Source = id

			// Call all callbacks
			m.mu.RLock()
			callbacks := make([]Callback, len(m.callbacks))
			copy(callbacks, m.callbacks)
			m.mu.RUnlock()

			for _, cb := range callbacks {
				// Execute with panic recovery
				func() {
					defer func() {
						if r := recover(); r != nil {
							_ = r // Log panic but don't crash.
						}
					}()
					_ = cb(m.ctx, change)
				}()
			}
		}
	}
}

// Stop stops all watchers.
func (m *Manager) Stop() error {
	if !m.running.Swap(false) {
		return nil // Not running
	}

	if m.cancel != nil {
		m.cancel()
	}

	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		// Timeout, force stop
	}

	// Stop all watchers
	m.mu.RLock()
	defer m.mu.RUnlock()

	var lastErr error
	for _, w := range m.watchers {
		if err := w.Stop(); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// Running returns true if the manager is running.
func (m *Manager) Running() bool {
	return m.running.Load()
}

// FileWatcher watches a file for changes using polling.
type FileWatcher struct {
	path     string
	interval time.Duration
	changes  chan Change
	lastMod  time.Time
	running  atomic.Bool
	stop     chan struct{}
	stopped  chan struct{}
}

// NewFileWatcher creates a new file watcher.
func NewFileWatcher(path string, opts ...FileWatcherOption) *FileWatcher {
	w := &FileWatcher{
		path:     path,
		interval: time.Second,
		changes:  make(chan Change, 10),
		stop:     make(chan struct{}),
		stopped:  make(chan struct{}),
	}

	for _, opt := range opts {
		opt(w)
	}

	return w
}

// FileWatcherOption configures a file watcher.
type FileWatcherOption func(*FileWatcher)

// WithInterval sets the polling interval.
func WithInterval(d time.Duration) FileWatcherOption {
	return func(w *FileWatcher) { w.interval = d }
}

// Start begins watching the file.
func (w *FileWatcher) Start(ctx context.Context) error {
	if w.running.Swap(true) {
		return nil // Already running
	}

	// Get initial file info
	info, err := getFileInfo(w.path)
	if err == nil {
		w.lastMod = info.ModTime
	}

	go w.watch(ctx)

	return nil
}

// watch polls the file for changes.
func (w *FileWatcher) watch(ctx context.Context) {
	defer close(w.stopped)
	defer close(w.changes)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stop:
			return
		case <-ticker.C:
			w.checkFile(ctx)
		}
	}
}

// checkFile checks if the file has changed.
func (w *FileWatcher) checkFile(ctx context.Context) {
	info, err := getFileInfo(w.path)
	if err != nil {
		// File might have been deleted
		select {
		case w.changes <- Change{
			Type:      ChangeDeleted,
			Timestamp: time.Now(),
			Error:     err,
		}:
		case <-ctx.Done():
		}

		return
	}

	if info.ModTime.After(w.lastMod) {
		w.lastMod = info.ModTime

		select {
		case w.changes <- Change{
			Type:      ChangeModified,
			Timestamp: time.Now(),
		}:
		case <-ctx.Done():
		}
	}
}

// Stop stops watching.
func (w *FileWatcher) Stop() error {
	if !w.running.Swap(false) {
		return nil
	}

	close(w.stop)
	<-w.stopped

	return nil
}

// Changes returns the change channel.
func (w *FileWatcher) Changes() <-chan Change {
	return w.changes
}

// FileInfo holds file information.
type FileInfo struct {
	Name    string
	Size    int64
	ModTime time.Time
}

// getFileInfo gets file information without importing os in the signature.
func getFileInfo(path string) (FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return FileInfo{}, err
	}

	return FileInfo{
		Name:    info.Name(),
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}, nil
}

// MultiWatcher combines multiple watchers.
type MultiWatcher struct {
	watchers []*FileWatcher
	changes  chan Change
	running  atomic.Bool
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewMultiWatcher creates a new multi-watcher.
func NewMultiWatcher(paths ...string) *MultiWatcher {
	w := &MultiWatcher{
		changes: make(chan Change, 20),
	}

	for _, path := range paths {
		w.watchers = append(w.watchers, NewFileWatcher(path))
	}

	return w
}

// Start starts all watchers.
func (w *MultiWatcher) Start(ctx context.Context) error {
	if w.running.Swap(true) {
		return nil
	}

	runCtx, cancel := context.WithCancel(ctx)
	w.ctx, w.cancel = runCtx, cancel

	for _, watcher := range w.watchers {
		if err := watcher.Start(runCtx); err != nil {
			if stopErr := w.Stop(); stopErr != nil {
				return stopErr
			}

			return err
		}

		w.wg.Add(1)
		go w.forward(watcher)
	}

	return nil
}

// forward forwards changes from a single watcher.
func (w *MultiWatcher) forward(watcher *FileWatcher) {
	defer w.wg.Done()

	for {
		select {
		case <-w.ctx.Done():
			return
		case change, ok := <-watcher.Changes():
			if !ok {
				return
			}

			select {
			case w.changes <- change:
			case <-w.ctx.Done():
				return
			}
		}
	}
}

// Stop stops all watchers.
func (w *MultiWatcher) Stop() error {
	if !w.running.Swap(false) {
		return nil
	}

	if w.cancel != nil {
		w.cancel()
	}

	// Stop all watchers
	for _, watcher := range w.watchers {
		if err := watcher.Stop(); err != nil {
			return err
		}
	}

	// Wait for forward goroutines
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}

	close(w.changes)

	return nil
}

// Changes returns the change channel.
func (w *MultiWatcher) Changes() <-chan Change {
	return w.changes
}

// Debouncer debounces change events.
type Debouncer struct {
	source   <-chan Change
	output   chan Change
	interval time.Duration
	running  atomic.Bool
	stop     chan struct{}
	stopped  chan struct{}
}

// NewDebouncer creates a new debouncer.
func NewDebouncer(source <-chan Change, interval time.Duration) *Debouncer {
	return &Debouncer{
		source:   source,
		output:   make(chan Change, 10),
		interval: interval,
		stop:     make(chan struct{}),
		stopped:  make(chan struct{}),
	}
}

// Start starts the debouncer.
func (d *Debouncer) Start(ctx context.Context) error {
	if d.running.Swap(true) {
		return nil
	}

	go d.run(ctx)

	return nil
}

// run processes and debounces changes.
func (d *Debouncer) run(ctx context.Context) {
	defer close(d.stopped)
	defer close(d.output)

	var pending *Change
	timer := time.NewTimer(d.interval)

	if !timer.Stop() {
		<-timer.C
	}
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stop:
			return
		case change, ok := <-d.source:
			if !ok {
				if pending != nil {
					select {
					case d.output <- *pending:
					default:
					}
				}

				return
			}

			pending = &change
			timer.Reset(d.interval)

		case <-timer.C:
			if pending != nil {
				select {
				case d.output <- *pending:
				case <-ctx.Done():
					return
				}
				pending = nil
			}
		}
	}
}

// Stop stops the debouncer.
func (d *Debouncer) Stop() error {
	if !d.running.Swap(false) {
		return nil
	}

	close(d.stop)
	<-d.stopped

	return nil
}

// Changes returns the debounced change channel.
func (d *Debouncer) Changes() <-chan Change {
	return d.output
}
