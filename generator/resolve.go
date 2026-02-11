// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

package generator

import "github.com/albertocavalcante/lspls/model"

// ResolveDeps expands a type filter to include all transitively
// referenced types from the model. Returns nil if filter is nil
// (meaning "generate all types").
//
// The includeProposed parameter controls whether proposed properties
// are followed during dependency walking.
func ResolveDeps(m *model.Model, filter map[string]bool, includeProposed bool) map[string]bool {
	if filter == nil {
		return nil
	}

	expanded := make(map[string]bool)
	for name := range filter {
		collectDeps(m, name, expanded, includeProposed)
	}
	return expanded
}

// collectDeps recursively collects all types referenced by typeName.
func collectDeps(m *model.Model, typeName string, visited map[string]bool, includeProposed bool) {
	if visited[typeName] {
		return // Already processed or cycle
	}
	visited[typeName] = true

	// Check structures
	for _, s := range m.Structures {
		if s.Name == typeName {
			for _, prop := range s.Properties {
				// Skip proposed properties when not including proposed types
				if prop.Proposed && !includeProposed {
					continue
				}
				collectTypeRefs(m, prop.Type, visited, includeProposed)
			}
			// Also check extends and mixins
			for _, ext := range s.Extends {
				collectTypeRefs(m, ext, visited, includeProposed)
			}
			for _, mix := range s.Mixins {
				collectTypeRefs(m, mix, visited, includeProposed)
			}
			return
		}
	}

	// Check type aliases
	for _, a := range m.TypeAliases {
		if a.Name == typeName {
			collectTypeRefs(m, a.Type, visited, includeProposed)
			return
		}
	}

	// Enums don't reference other types, nothing to do
}

// collectTypeRefs extracts type references from a Type and recursively
// collects their dependencies.
func collectTypeRefs(m *model.Model, t *model.Type, visited map[string]bool, includeProposed bool) {
	if t == nil {
		return
	}
	switch t.Kind {
	case "reference":
		collectDeps(m, t.Name, visited, includeProposed)
	case "array":
		collectTypeRefs(m, t.Element, visited, includeProposed)
	case "map":
		collectTypeRefs(m, t.Key, visited, includeProposed)
		if vt, ok := t.Value.(*model.Type); ok {
			collectTypeRefs(m, vt, visited, includeProposed)
		}
	case "or":
		for _, item := range t.Items {
			collectTypeRefs(m, item, visited, includeProposed)
		}
	case "and":
		for _, item := range t.Items {
			collectTypeRefs(m, item, visited, includeProposed)
		}
	case "tuple":
		for _, item := range t.Items {
			collectTypeRefs(m, item, visited, includeProposed)
		}
	case "literal":
		// Literal types have inline properties
		if lit, ok := t.Value.(model.Literal); ok {
			for _, prop := range lit.Properties {
				collectTypeRefs(m, prop.Type, visited, includeProposed)
			}
		}
	}
}
