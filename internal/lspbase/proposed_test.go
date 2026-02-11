// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

package lspbase

import "testing"

func TestProposedTypes(t *testing.T) {
	got := ProposedTypes(
		NamedProposal{Name: "Position", Proposed: false},
		NamedProposal{Name: "InlayHint", Proposed: true},
		NamedProposal{Name: "InlayHintKind", Proposed: true},
		NamedProposal{Name: "DiagnosticSeverity", Proposed: false},
		NamedProposal{Name: "DocumentUri", Proposed: false},
	)

	tests := []struct {
		name string
		want bool
	}{
		{name: "Position", want: false},
		{name: "InlayHint", want: true},
		{name: "InlayHintKind", want: true},
		{name: "DiagnosticSeverity", want: false},
		{name: "DocumentUri", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if val, ok := got[tt.name]; !ok {
				t.Errorf("ProposedTypes missing key %q", tt.name)
			} else if val != tt.want {
				t.Errorf("ProposedTypes[%q] = %v, want %v", tt.name, val, tt.want)
			}
		})
	}

	// Test that unknown type is not present.
	if _, ok := got["Unknown"]; ok {
		t.Error("ProposedTypes should not contain 'Unknown'")
	}
}

func TestProposedTypesEmpty(t *testing.T) {
	got := ProposedTypes()
	if len(got) != 0 {
		t.Errorf("ProposedTypes() with no args should return empty map, got %v", got)
	}
}
