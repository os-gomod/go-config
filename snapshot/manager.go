package snapshot

import "time"

// Snapshot represents a config snapshot.
type Snapshot struct {
	Version int
	At      time.Time
	Data    map[string]any
}

// Manager manages snapshots.
type Manager struct {
	max   int
	store []Snapshot
}

func NewManager(maxSnapshots int) *Manager {
	return &Manager{
		max:   maxSnapshots,
		store: make([]Snapshot, 0, maxSnapshots),
	}
}

func (m *Manager) Take(data map[string]any) Snapshot {
	s := Snapshot{
		Version: len(m.store) + 1,
		At:      time.Now(),
		Data:    data,
	}

	m.store = append(m.store, s)
	m.trimStore()
	return s
}

func (m *Manager) trimStore() {
	if len(m.store) > m.max {
		m.store = m.store[1:]
	}
}

func (m *Manager) Get(version int) (Snapshot, bool) {
	for _, s := range m.store {
		if s.Version == version {
			return s, true
		}
	}
	return Snapshot{}, false
}
