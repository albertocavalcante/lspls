// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

package generator

import (
	"fmt"
	"slices"
	"sync"
)

var (
	mu       sync.RWMutex
	registry = make(map[string]Generator)
)

// Register adds a generator to the registry.
func Register(g Generator) {
	mu.Lock()
	defer mu.Unlock()
	meta := g.Metadata()
	if _, exists := registry[meta.Name]; exists {
		panic(fmt.Sprintf("generator %q already registered", meta.Name))
	}
	registry[meta.Name] = g
}

// Get returns a generator by name.
func Get(name string) (Generator, bool) {
	mu.RLock()
	defer mu.RUnlock()
	g, ok := registry[name]
	return g, ok
}

// List returns all registered generator names, sorted.
func List() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// All returns all registered generators.
func All() []Generator {
	mu.RLock()
	defer mu.RUnlock()
	gens := make([]Generator, 0, len(registry))
	for _, g := range registry {
		gens = append(gens, g)
	}
	return gens
}

// Reset clears the registry (for testing).
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	registry = make(map[string]Generator)
}
