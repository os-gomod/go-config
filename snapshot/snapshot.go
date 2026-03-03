// Package snapshot provides immutable configuration snapshots.
// Supports time-travel queries and recovery operations.
package snapshot

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/os-gomod/go-config/types"
)

// Snapshot represents an immutable configuration state at a point in time.
type Snapshot struct {
	id        uint64
	data      map[string]types.Value
	timestamp time.Time
	metadata  *Metadata
	checksum  [16]byte
}

// Metadata contains snapshot metadata.
type Metadata struct {
	Source    string            `json:"source"`
	Version   uint64            `json:"version"`
	Tags      map[string]string `json:"tags,omitempty"`
	CreatedBy string            `json:"created_by,omitempty"`
}

// ID returns the snapshot ID.
func (s *Snapshot) ID() uint64 { return s.id }

// Data returns a copy of the snapshot data.
func (s *Snapshot) Data() map[string]types.Value {
	result := make(map[string]types.Value, len(s.data))
	for k, v := range s.data {
		result[k] = v
	}

	return result
}

// Timestamp returns when the snapshot was taken.
func (s *Snapshot) Timestamp() time.Time { return s.timestamp }

// Metadata returns the snapshot metadata.
func (s *Snapshot) Metadata() *Metadata { return s.metadata }

// Checksum returns the snapshot checksum.
func (s *Snapshot) Checksum() [16]byte { return s.checksum }

// Get retrieves a value from the snapshot.
func (s *Snapshot) Get(key string) (types.Value, bool) {
	v, ok := s.data[key]

	return v, ok
}

// MarshalJSON implements JSON marshaling.
func (s *Snapshot) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID        uint64         `json:"id"`
		Timestamp time.Time      `json:"timestamp"`
		Data      map[string]any `json:"data"`
		Metadata  *Metadata      `json:"metadata,omitempty"`
	}{
		ID:        s.id,
		Timestamp: s.timestamp,
		Data:      flattenData(s.data),
		Metadata:  s.metadata,
	})
}

// flattenData converts typed values to raw values.
func flattenData(data map[string]types.Value) map[string]any {
	result := make(map[string]any, len(data))
	for k, v := range data {
		result[k] = v.Raw()
	}

	return result
}

// Manager manages configuration snapshots.
type Manager struct {
	snapshots []*Snapshot
	maxSize   int
	nextID    atomic.Uint64
	mu        sync.RWMutex
}

// NewManager creates a new snapshot manager.
func NewManager(maxSize int) *Manager {
	if maxSize <= 0 {
		maxSize = 100 // Default
	}

	return &Manager{
		snapshots: make([]*Snapshot, 0, maxSize),
		maxSize:   maxSize,
	}
}

// Take creates a new snapshot from the current state.
func (m *Manager) Take(data map[string]types.Value, opts ...SnapshotOption) *Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := m.nextID.Add(1)

	s := &Snapshot{
		id:        id,
		data:      data,
		timestamp: time.Now(),
		metadata:  &Metadata{Version: id},
		checksum:  computeChecksum(data),
	}

	for _, opt := range opts {
		opt(s)
	}

	// Add to snapshots
	m.snapshots = append(m.snapshots, s)

	// Enforce max size
	if len(m.snapshots) > m.maxSize {
		m.snapshots = m.snapshots[1:]
	}

	return s
}

// SnapshotOption configures snapshot creation.
type SnapshotOption func(*Snapshot)

// WithMetadata sets snapshot metadata.
func WithMetadata(meta *Metadata) SnapshotOption {
	return func(s *Snapshot) { s.metadata = meta }
}

// WithSource sets the snapshot source.
func WithSource(source string) SnapshotOption {
	return func(s *Snapshot) { s.metadata.Source = source }
}

// WithTags sets snapshot tags.
func WithTags(tags map[string]string) SnapshotOption {
	return func(s *Snapshot) {
		if s.metadata.Tags == nil {
			s.metadata.Tags = make(map[string]string)
		}
		for k, v := range tags {
			s.metadata.Tags[k] = v
		}
	}
}

// Get retrieves a snapshot by ID.
func (m *Manager) Get(id uint64) (*Snapshot, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Binary search for efficiency
	left, right := 0, len(m.snapshots)-1
	for left <= right {
		mid := (left + right) / 2
		if m.snapshots[mid].id == id {
			return m.snapshots[mid], true
		}
		if m.snapshots[mid].id < id {
			left = mid + 1
		} else {
			right = mid - 1
		}
	}

	return nil, false
}

// Latest returns the most recent snapshot.
func (m *Manager) Latest() *Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.snapshots) == 0 {
		return nil
	}

	return m.snapshots[len(m.snapshots)-1]
}

// At returns the snapshot closest to the given time.
func (m *Manager) At(t time.Time) *Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.snapshots) == 0 {
		return nil
	}

	// Binary search for closest timestamp
	left, right := 0, len(m.snapshots)-1

	for left < right {
		mid := (left + right + 1) / 2
		if m.snapshots[mid].timestamp.After(t) {
			right = mid - 1
		} else {
			left = mid
		}
	}

	return m.snapshots[left]
}

// List returns all snapshots within a time range.
func (m *Manager) List(start, end time.Time) []*Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Snapshot, 0)

	for _, s := range m.snapshots {
		if (start.IsZero() || !s.timestamp.Before(start)) &&
			(end.IsZero() || !s.timestamp.After(end)) {
			result = append(result, s)
		}
	}

	return result
}

// Count returns the number of stored snapshots.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.snapshots)
}

// Clear removes all snapshots.
func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snapshots = m.snapshots[:0]
}

// Prune removes snapshots older than a duration.
func (m *Manager) Prune(olderThan time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)
	pruned := 0

	// Find first index to keep
	firstToKeep := 0
	for i, s := range m.snapshots {
		if s.timestamp.After(cutoff) {
			firstToKeep = i

			break
		}
		pruned++
	}

	if pruned > 0 {
		m.snapshots = m.snapshots[firstToKeep:]
	}

	return pruned
}

// Diff compares two snapshots and returns differences.
func Diff(a, b *Snapshot) *DiffResult {
	result := &DiffResult{
		Created:   make(map[string]types.Value),
		Updated:   make(map[string]DiffPair),
		Deleted:   make(map[string]types.Value),
		Unchanged: make(map[string]types.Value),
	}

	// Check for updates and deletes
	for k, av := range a.data {
		if bv, ok := b.data[k]; ok {
			if av.Raw() != bv.Raw() || av.Type() != bv.Type() {
				result.Updated[k] = DiffPair{Old: av, New: bv}
			} else {
				result.Unchanged[k] = av
			}
		} else {
			result.Deleted[k] = av
		}
	}

	// Check for creates
	for k, bv := range b.data {
		if _, ok := a.data[k]; !ok {
			result.Created[k] = bv
		}
	}

	return result
}

// DiffResult contains the result of a snapshot comparison.
type DiffResult struct {
	Created   map[string]types.Value
	Updated   map[string]DiffPair
	Deleted   map[string]types.Value
	Unchanged map[string]types.Value
}

// DiffPair contains old and new values for an update.
type DiffPair struct {
	Old types.Value
	New types.Value
}

// HasChanges returns true if there are any changes.
func (d *DiffResult) HasChanges() bool {
	return len(d.Created) > 0 || len(d.Updated) > 0 || len(d.Deleted) > 0
}

// computeChecksum computes a simple checksum for the snapshot.
func computeChecksum(data map[string]types.Value) [16]byte {
	var sum [16]byte

	// Simple checksum based on data size and hash
	for k, v := range data {
		// XOR key bytes
		for i, b := range []byte(k) {
			sum[i%16] ^= b
		}
		// Mix in value hash
		if s, ok := v.Raw().(string); ok {
			for i, b := range []byte(s) {
				sum[(i+8)%16] ^= b
			}
		}
	}

	return sum
}

// Store provides persistent snapshot storage.
type Store interface {
	Save(ctx context.Context, snapshot *Snapshot) error
	Load(ctx context.Context, id uint64) (*Snapshot, error)
	List(ctx context.Context, opts ListOptions) ([]*Snapshot, error)
	Delete(ctx context.Context, id uint64) error
}

// ListOptions options for listing snapshots.
type ListOptions struct {
	Start  time.Time
	End    time.Time
	Tag    string
	Limit  int
	Offset int
}

// MemoryStore is an in-memory snapshot store.
type MemoryStore struct {
	snapshots map[uint64]*Snapshot
	mu        sync.RWMutex
}

// NewMemoryStore creates a new memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		snapshots: make(map[uint64]*Snapshot),
	}
}

// Save stores a snapshot.
func (s *MemoryStore) Save(_ context.Context, snapshot *Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Copy the snapshot
	data := make(map[string]types.Value, len(snapshot.data))
	for k, v := range snapshot.data {
		data[k] = v
	}

	s.snapshots[snapshot.id] = &Snapshot{
		id:        snapshot.id,
		data:      data,
		timestamp: snapshot.timestamp,
		metadata:  snapshot.metadata,
		checksum:  snapshot.checksum,
	}

	return nil
}

// Load retrieves a snapshot by ID.
func (s *MemoryStore) Load(_ context.Context, id uint64) (*Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot, ok := s.snapshots[id]
	if !ok {
		return nil, types.NewError(types.ErrNotFound, "snapshot not found")
	}

	return snapshot, nil
}

// List returns snapshots matching the options.
func (s *MemoryStore) List(_ context.Context, opts ListOptions) ([]*Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Snapshot, 0)

	for _, snapshot := range s.snapshots {
		// Filter by time range
		if !opts.Start.IsZero() && snapshot.timestamp.Before(opts.Start) {
			continue
		}
		if !opts.End.IsZero() && snapshot.timestamp.After(opts.End) {
			continue
		}

		// Filter by tag
		if opts.Tag != "" {
			if snapshot.metadata == nil || snapshot.metadata.Tags == nil {
				continue
			}
			if _, ok := snapshot.metadata.Tags[opts.Tag]; !ok {
				continue
			}
		}

		result = append(result, snapshot)
	}

	// Apply offset and limit
	if opts.Offset > 0 {
		if opts.Offset >= len(result) {
			return nil, nil
		}
		result = result[opts.Offset:]
	}

	if opts.Limit > 0 && len(result) > opts.Limit {
		result = result[:opts.Limit]
	}

	return result, nil
}

// Delete removes a snapshot.
func (s *MemoryStore) Delete(_ context.Context, id uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.snapshots, id)

	return nil
}
