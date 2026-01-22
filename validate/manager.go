package validate

import "sync"

// Manager orchestrates validation rules.
type Manager struct {
	mu          sync.RWMutex
	rules       map[string]Rule
	structRules []StructRule
}

func NewManager() *Manager {
	return &Manager{rules: make(map[string]Rule)}
}

// Register adds a validation rule.
func (m *Manager) Register(key string, rule Rule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules[key] = rule
}

// RegisterStructRule adds a struct-level validation rule.
func (m *Manager) RegisterStructRule(r StructRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.structRules = append(m.structRules, r)
}

// Validate validates config data.
func (m *Manager) Validate(data map[string]any) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	errs := make(map[string]error)
	for key, rule := range m.rules {
		if err := rule.Validate(key, data); err != nil {
			errs[key] = err
		}
	}

	if len(errs) > 0 {
		return ValidationErrors{Errors: errs}
	}
	return nil
}

// ValidateStruct validates a struct using registered struct rules.
func (m *Manager) ValidateStruct(s any) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, r := range m.structRules {
		if err := r.ValidateStruct(s); err != nil {
			return err
		}
	}
	return nil
}
