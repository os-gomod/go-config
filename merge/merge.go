// Package merge provides configuration merging utilities.
// Implements priority-based merging with conflict resolution.
package merge

import (
	"sort"

	"github.com/os-gomod/go-config/types"
)

// Strategy defines the merge strategy.
type Strategy int

const (
	// StrategyPriority uses source priority (higher wins).
	StrategyPriority Strategy = iota
	// StrategyFirst keeps the first encountered value.
	StrategyFirst
	// StrategyLast keeps the last encountered value.
	StrategyLast
	// StrategyDeep performs deep merge for nested structures.
	StrategyDeep
)

// Merger combines multiple configuration maps.
type Merger struct {
	strategy Strategy
}

// NewMerger creates a new merger with the given strategy.
func NewMerger(strategy Strategy) *Merger {
	return &Merger{strategy: strategy}
}

// Merge combines multiple configuration maps according to the strategy.
func (m *Merger) Merge(sources ...map[string]types.Value) map[string]types.Value {
	switch m.strategy {
	case StrategyPriority:
		return m.mergePriority(sources...)
	case StrategyFirst:
		return m.mergeFirst(sources...)
	case StrategyLast:
		return m.mergeLast(sources...)
	case StrategyDeep:
		return m.mergeDeep(sources...)
	default:
		return m.mergePriority(sources...)
	}
}

// mergePriority uses source priority for conflict resolution.
func (m *Merger) mergePriority(sources ...map[string]types.Value) map[string]types.Value {
	if len(sources) == 0 {
		return nil
	}
	if len(sources) == 1 {
		return sources[0]
	}

	result := make(map[string]types.Value)

	for _, source := range sources {
		for k, v := range source {
			if existing, ok := result[k]; !ok || v.Priority() > existing.Priority() {
				result[k] = v
			}
		}
	}

	return result
}

// mergeFirst keeps the first encountered value.
func (m *Merger) mergeFirst(sources ...map[string]types.Value) map[string]types.Value {
	if len(sources) == 0 {
		return nil
	}
	if len(sources) == 1 {
		return sources[0]
	}

	result := make(map[string]types.Value)

	for _, source := range sources {
		for k, v := range source {
			if _, ok := result[k]; !ok {
				result[k] = v
			}
		}
	}

	return result
}

// mergeLast keeps the last encountered value.
func (m *Merger) mergeLast(sources ...map[string]types.Value) map[string]types.Value {
	if len(sources) == 0 {
		return nil
	}
	if len(sources) == 1 {
		return sources[0]
	}

	result := make(map[string]types.Value)

	for _, source := range sources {
		for k, v := range source {
			result[k] = v
		}
	}

	return result
}

// mergeDeep performs deep merge for nested structures.
func (m *Merger) mergeDeep(sources ...map[string]types.Value) map[string]types.Value {
	if len(sources) == 0 {
		return nil
	}
	if len(sources) == 1 {
		return sources[0]
	}

	// Build nested structure from all sources
	nested := make(map[string]any)

	for _, source := range sources {
		for k, v := range source {
			insertNested(nested, k, v)
		}
	}

	// Flatten back to dotted keys
	return flattenValues(nested, "")
}

// insertNested inserts a value into a nested map structure.
func insertNested(m map[string]any, key string, value types.Value) {
	parts := splitKey(key)
	current := m

	for i, part := range parts {
		if i == len(parts)-1 {
			// Last part, store the value
			if existing, ok := current[part]; ok {
				// Merge if both are maps
				if existingMap, ok := existing.(map[string]any); ok {
					if newMap, ok := value.Raw().(map[string]any); ok {
						for k, v := range newMap {
							existingMap[k] = v
						}

						return
					}
				}
			}
			current[part] = value

			return
		}

		if _, ok := current[part]; !ok {
			current[part] = make(map[string]any)
		}

		if next, ok := current[part].(map[string]any); ok {
			current = next
		} else {
			newMap := make(map[string]any)
			current[part] = newMap
			current = newMap
		}
	}
}

// flattenValues flattens a nested map to dotted keys.
func flattenValues(m map[string]any, prefix string) map[string]types.Value {
	result := make(map[string]types.Value)

	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}

		switch val := v.(type) {
		case types.Value:
			result[key] = val
		case map[string]any:
			for k2, v2 := range flattenValues(val, key) {
				result[k2] = v2
			}
		default:
			result[key] = types.NewValue(val, types.TypeUnknown, types.SourceMemory, 0)
		}
	}

	return result
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

// SortedSources sorts sources by priority.
type SortedSources []map[string]types.Value

// Len implements sort.Interface.
func (s SortedSources) Len() int { return len(s) }

// Less implements sort.Interface.
func (s SortedSources) Less(i, j int) bool {
	// Compare highest priority values from each source
	pri, prj := 0, 0
	for _, v := range s[i] {
		if v.Priority() > pri {
			pri = v.Priority()
		}
	}
	for _, v := range s[j] {
		if v.Priority() > prj {
			prj = v.Priority()
		}
	}

	return pri < prj
}

// Swap implements sort.Interface.
func (s SortedSources) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// SortByPriority sorts sources by their priority.
func SortByPriority(sources []map[string]types.Value) {
	sort.Sort(SortedSources(sources))
}

// Conflict represents a merge conflict.
type Conflict struct {
	Key      string
	Values   []types.Value
	Resolved types.Value
	Strategy Strategy
}

// Resolver handles conflict resolution.
type Resolver struct {
	conflicts []Conflict
}

// NewResolver creates a new conflict resolver.
func NewResolver() *Resolver {
	return &Resolver{
		conflicts: make([]Conflict, 0),
	}
}

// Record records a conflict for later resolution.
func (r *Resolver) Record(key string, values []types.Value, strategy Strategy) {
	resolved := r.resolve(key, values, strategy)
	r.conflicts = append(r.conflicts, Conflict{
		Key:      key,
		Values:   values,
		Resolved: resolved,
		Strategy: strategy,
	})
}

// resolve resolves a conflict according to the strategy.
func (r *Resolver) resolve(key string, values []types.Value, strategy Strategy) types.Value {
	if len(values) == 0 {
		return types.Value{}
	}
	if len(values) == 1 {
		return values[0]
	}

	switch strategy {
	case StrategyPriority:
		// Find highest priority
		max := values[0]
		for _, v := range values[1:] {
			if v.Priority() > max.Priority() {
				max = v
			}
		}

		return max
	case StrategyFirst:
		return values[0]
	case StrategyLast:
		return values[len(values)-1]
	default:
		return values[0]
	}
}

// Conflicts returns all recorded conflicts.
func (r *Resolver) Conflicts() []Conflict {
	return r.conflicts
}

// HasConflicts returns true if there are any conflicts.
func (r *Resolver) HasConflicts() bool {
	return len(r.conflicts) > 0
}

// Clear clears all recorded conflicts.
func (r *Resolver) Clear() {
	r.conflicts = r.conflicts[:0]
}
