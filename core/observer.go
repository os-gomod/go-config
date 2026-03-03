// Package core provides the observer management system.
// Implements a non-blocking fan-out model with bounded queues.
package core

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/os-gomod/go-config/types"
)

// ObserverManager handles event distribution to observers.
// Uses a fan-out model with bounded queues to prevent blocking.
type ObserverManager struct {
	// subscribers holds all active subscriptions.
	subscribers sync.Map // map[int64]*subscription

	// nextID generates unique subscription IDs.
	nextID atomic.Int64

	// queueSize is the bounded queue size per subscriber.
	queueSize int

	// closed indicates if the manager is shut down.
	closed atomic.Bool

	// wg tracks active goroutines for graceful shutdown.
	wg sync.WaitGroup
}

// subscription represents an active observer subscription.
type subscription struct {
	id       int64
	observer types.Observer
	queue    chan types.Event
	cancel   context.CancelFunc
	active   atomic.Bool
}

// NewObserverManager creates a new observer manager.
func NewObserverManager(queueSize int) *ObserverManager {
	return &ObserverManager{
		queueSize: queueSize,
	}
}

// Subscribe registers an observer and returns a cancel function.
func (m *ObserverManager) Subscribe(ctx context.Context, observer types.Observer) (func(), error) {
	if m.closed.Load() {
		return nil, types.NewError(types.ErrContextCanceled, "observer manager is closed")
	}

	id := m.nextID.Add(1)

	ctx, cancel := context.WithCancel(ctx)

	sub := &subscription{
		id:       id,
		observer: observer,
		queue:    make(chan types.Event, m.queueSize),
		cancel:   cancel,
	}
	sub.active.Store(true)

	m.subscribers.Store(id, sub)

	// Start event processor goroutine
	m.wg.Add(1)
	go m.processEvents(ctx, sub)

	// Return cancel function
	return func() {
		m.unsubscribe(id)
	}, nil
}

// unsubscribe removes a subscription.
func (m *ObserverManager) unsubscribe(id int64) {
	if v, ok := m.subscribers.LoadAndDelete(id); ok {
		sub := v.(*subscription)
		sub.active.Store(false)
		sub.cancel()
	}
}

// Notify sends an event to all subscribers.
// Non-blocking: drops events if subscriber queue is full.
func (m *ObserverManager) Notify(_ context.Context, event types.Event) {
	if m.closed.Load() {
		return
	}

	m.subscribers.Range(func(_, value any) bool {
		sub := value.(*subscription)

		if !sub.active.Load() {
			return true
		}

		// Non-blocking send with fallback
		select {
		case sub.queue <- event:
			// Event queued successfully
		default:
			// Queue full, drop event to prevent blocking
			// In production, this should be monitored/metric'd
		}

		return true
	})
}

// processEvents processes events for a subscription.
func (m *ObserverManager) processEvents(ctx context.Context, sub *subscription) {
	defer m.wg.Done()
	defer close(sub.queue)

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-sub.queue:
			if !ok {
				return
			}

			// Execute observer with panic recovery
			func() {
				defer func() {
					if r := recover(); r != nil {
						_ = r // Log panic but don't crash.
					}
				}()

				_ = sub.observer(ctx, event)
			}()
		}
	}
}

// Close shuts down the observer manager.
func (m *ObserverManager) Close(ctx context.Context) error {
	if m.closed.Swap(true) {
		return nil // Already closed
	}

	// Cancel all subscriptions
	m.subscribers.Range(func(_, value any) bool {
		sub := value.(*subscription)
		sub.active.Store(false)
		sub.cancel()

		return true
	})

	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stats returns observer statistics.
func (m *ObserverManager) Stats() ObserverStats {
	var count int
	var totalQueueLen int

	m.subscribers.Range(func(_, value any) bool {
		count++
		sub := value.(*subscription)
		totalQueueLen += len(sub.queue)

		return true
	})

	return ObserverStats{
		ActiveSubscribers: count,
		TotalQueueSize:    totalQueueLen,
	}
}

// ObserverStats contains observer manager statistics.
type ObserverStats struct {
	ActiveSubscribers int
	TotalQueueSize    int
}
