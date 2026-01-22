package util

import "reflect"

// Clone returns a shallow copy of a map.
func Clone(src map[string]any) map[string]any {
	return copyMap(src)
}

// Merge merges src into dst (dst wins on conflicts).
func Merge(dst, src map[string]any) {
	for k, v := range src {
		dst[k] = v
	}
}

// Keys returns all keys in a map.
func Keys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// DeepEqual compares two values deeply.
func DeepEqual(a, b any) bool {
	return reflect.DeepEqual(a, b)
}

func copyMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
