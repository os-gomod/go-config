// Package syncutil provides synchronization utilities for the configuration system.
package syncutil

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// OnceWithErr is like sync.Once but can return an error.
type OnceWithErr struct {
	once sync.Once
	err  error
}

// Do executes the function once and caches the error.
func (o *OnceWithErr) Do(fn func() error) error {
	o.once.Do(func() {
		o.err = fn()
	})

	return o.err
}

// Reset resets the OnceWithErr to allow another execution.
func (o *OnceWithErr) Reset() {
	o.once = sync.Once{}
	o.err = nil
}

// WaitGroup with context support.
type WaitGroup struct {
	wg sync.WaitGroup
}

// Add adds delta to the WaitGroup counter.
func (wg *WaitGroup) Add(delta int) {
	wg.wg.Add(delta)
}

// Done decrements the WaitGroup counter.
func (wg *WaitGroup) Done() {
	wg.wg.Done()
}

// Wait blocks until the WaitGroup counter is zero.
func (wg *WaitGroup) Wait() {
	wg.wg.Wait()
}

// WaitContext blocks until the WaitGroup counter is zero or context is done.
func (wg *WaitGroup) WaitContext(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		wg.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// WaitTimeout blocks until the WaitGroup counter is zero or timeout.
func (wg *WaitGroup) WaitTimeout(timeout time.Duration) error {
	done := make(chan struct{})
	go func() {
		wg.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return context.DeadlineExceeded
	}
}

// AtomicBool is an atomic boolean.
type AtomicBool struct {
	value atomic.Bool
}

// NewAtomicBool creates a new AtomicBool.
func NewAtomicBool(initial bool) *AtomicBool {
	ab := &AtomicBool{}
	ab.value.Store(initial)

	return ab
}

// Set sets the boolean value.
func (ab *AtomicBool) Set(value bool) {
	ab.value.Store(value)
}

// Get gets the boolean value.
func (ab *AtomicBool) Get() bool {
	return ab.value.Load()
}

// Swap sets the boolean value and returns the old value.
func (ab *AtomicBool) Swap(new bool) bool {
	return ab.value.Swap(new)
}

// CompareAndSwap performs an atomic compare-and-swap.
func (ab *AtomicBool) CompareAndSwap(old, new bool) bool {
	return ab.value.CompareAndSwap(old, new)
}

// AtomicInt64 is an atomic int64.
type AtomicInt64 struct {
	value atomic.Int64
}

// NewAtomicInt64 creates a new AtomicInt64.
func NewAtomicInt64(initial int64) *AtomicInt64 {
	ai := &AtomicInt64{}
	ai.value.Store(initial)

	return ai
}

// Set sets the int64 value.
func (ai *AtomicInt64) Set(value int64) {
	ai.value.Store(value)
}

// Get gets the int64 value.
func (ai *AtomicInt64) Get() int64 {
	return ai.value.Load()
}

// Add adds delta to the int64 value.
func (ai *AtomicInt64) Add(delta int64) int64 {
	return ai.value.Add(delta)
}

// Increment increments the int64 value by 1.
func (ai *AtomicInt64) Increment() int64 {
	return ai.value.Add(1)
}

// Decrement decrements the int64 value by 1.
func (ai *AtomicInt64) Decrement() int64 {
	return ai.value.Add(-1)
}

// Swap sets the int64 value and returns the old value.
func (ai *AtomicInt64) Swap(new int64) int64 {
	return ai.value.Swap(new)
}

// CompareAndSwap performs an atomic compare-and-swap.
func (ai *AtomicInt64) CompareAndSwap(old, new int64) bool {
	return ai.value.CompareAndSwap(old, new)
}

// Mutex with try-lock support.
type Mutex struct {
	mu sync.Mutex
}

// Lock locks the mutex.
func (m *Mutex) Lock() {
	m.mu.Lock()
}

// Unlock unlocks the mutex.
func (m *Mutex) Unlock() {
	m.mu.Unlock()
}

// TryLock tries to lock the mutex without blocking.
func (m *Mutex) TryLock() bool {
	return m.mu.TryLock()
}

// LockContext locks the mutex with context support.
func (m *Mutex) LockContext(ctx context.Context) error {
	if m.TryLock() {
		return nil
	}

	done := make(chan struct{})
	go func() {
		m.mu.Lock()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		go func() {
			<-done
			m.mu.Unlock()
		}()

		return ctx.Err()
	}
}

// RWMutex with try-lock support.
type RWMutex struct {
	mu sync.RWMutex
}

// Lock locks the mutex for writing.
func (m *RWMutex) Lock() {
	m.mu.Lock()
}

// Unlock unlocks the mutex for writing.
func (m *RWMutex) Unlock() {
	m.mu.Unlock()
}

// RLock locks the mutex for reading.
func (m *RWMutex) RLock() {
	m.mu.RLock()
}

// RUnlock unlocks the mutex for reading.
func (m *RWMutex) RUnlock() {
	m.mu.RUnlock()
}

// TryLock tries to lock the mutex for writing without blocking.
func (m *RWMutex) TryLock() bool {
	return m.mu.TryLock()
}

// TryRLock tries to lock the mutex for reading without blocking.
func (m *RWMutex) TryRLock() bool {
	return m.mu.TryRLock()
}

// Semaphore is a counting semaphore.
type Semaphore struct {
	ch chan struct{}
}

// NewSemaphore creates a new semaphore with the given capacity.
func NewSemaphore(capacity int) *Semaphore {
	return &Semaphore{
		ch: make(chan struct{}, capacity),
	}
}

// Acquire acquires a permit.
func (s *Semaphore) Acquire(ctx context.Context) error {
	select {
	case s.ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release releases a permit.
func (s *Semaphore) Release() {
	<-s.ch
}

// TryAcquire tries to acquire a permit without blocking.
func (s *Semaphore) TryAcquire() bool {
	select {
	case s.ch <- struct{}{}:
		return true
	default:
		return false
	}
}

// Latch is a one-time signal.
type Latch struct {
	ch   chan struct{}
	once sync.Once
}

// NewLatch creates a new latch.
func NewLatch() *Latch {
	return &Latch{
		ch: make(chan struct{}),
	}
}

// Signal signals the latch.
func (l *Latch) Signal() {
	l.once.Do(func() {
		close(l.ch)
	})
}

// Wait waits for the latch to be signaled.
func (l *Latch) Wait() {
	<-l.ch
}

// WaitContext waits for the latch with context support.
func (l *Latch) WaitContext(ctx context.Context) error {
	select {
	case <-l.ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// IsSignaled returns true if the latch has been signaled.
func (l *Latch) IsSignaled() bool {
	select {
	case <-l.ch:
		return true
	default:
		return false
	}
}

// Barrier is a reusable barrier for goroutine synchronization.
type Barrier struct {
	n     int
	count atomic.Int32
	mu    sync.Mutex
	ch    chan struct{}
}

// NewBarrier creates a new barrier for n goroutines.
func NewBarrier(n int) *Barrier {
	return &Barrier{
		n:  n,
		ch: make(chan struct{}),
	}
}

// Wait blocks until all goroutines have called Wait.
func (b *Barrier) Wait() {
	count := b.count.Add(1)

	if int(count) == b.n {
		// Last goroutine, release all
		close(b.ch)
		b.mu.Lock()
		b.ch = make(chan struct{})
		b.count.Store(0)
		b.mu.Unlock()

		return
	}

	// Wait for release
	<-b.ch
}

// ContextGuard manages context lifecycle.
type ContextGuard struct {
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
	done   atomic.Bool
}

// NewContextGuard creates a new context guard.
func NewContextGuard(parent context.Context) *ContextGuard {
	ctx, cancel := context.WithCancel(parent)

	return &ContextGuard{
		ctx:    ctx,
		cancel: cancel,
	}
}

// Context returns the guarded context.
func (g *ContextGuard) Context() context.Context {
	return g.ctx
}

// Done returns true if the context is done.
func (g *ContextGuard) Done() bool {
	return g.done.Load()
}

// Cancel cancels the context.
func (g *ContextGuard) Cancel() {
	if g.done.CompareAndSwap(false, true) {
		g.cancel()
	}
}

// Wait blocks until the context is done.
func (g *ContextGuard) Wait() {
	<-g.ctx.Done()
}
