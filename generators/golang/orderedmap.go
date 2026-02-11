// SPDX-License-Identifier: MIT

package golang

import "slices"

// orderedMap maintains insertion order for deterministic output.
type orderedMap[T any] struct {
	m     map[string]T
	order []string
}

func newOrderedMap[T any]() *orderedMap[T] {
	return &orderedMap[T]{
		m: make(map[string]T),
	}
}

func (m *orderedMap[T]) set(key string, value T) {
	if _, exists := m.m[key]; !exists {
		m.order = append(m.order, key)
	}
	m.m[key] = value
}

func (m *orderedMap[T]) get(key string) T {
	return m.m[key]
}

func (m *orderedMap[T]) keys() []string {
	// Sort for deterministic output
	sorted := slices.Clone(m.order)
	slices.Sort(sorted)
	return sorted
}
