// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

package lspbase

// NamedProposal pairs a type name with its proposed status.
type NamedProposal struct {
	Name     string
	Proposed bool
}

// ProposedTypes returns a map from type name to its proposed status.
// Call it with slices of NamedProposal built from structures, enumerations,
// and type aliases. This avoids importing the model package.
func ProposedTypes(items ...NamedProposal) map[string]bool {
	cache := make(map[string]bool, len(items))
	for _, p := range items {
		cache[p.Name] = p.Proposed
	}
	return cache
}
