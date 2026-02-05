// SPDX-License-Identifier: MIT

package codegen

import (
	"sort"
	"testing"

	"github.com/albertocavalcante/lspls/internal/model"
)

func TestResolveTransitiveDeps(t *testing.T) {
	tests := []struct {
		name   string
		model  *model.Model
		filter []string
		want   []string // expected types after resolution
	}{
		{
			name: "no filter returns nil",
			model: &model.Model{
				Structures: []*model.Structure{
					{Name: "Position"},
				},
			},
			filter: nil,
			want:   nil, // nil means all types
		},
		{
			name: "struct referencing struct",
			model: &model.Model{
				Structures: []*model.Structure{
					{
						Name: "Position",
						Properties: []model.Property{
							{Name: "line", Type: &model.Type{Kind: "base", Name: "uinteger"}},
						},
					},
					{
						Name: "Range",
						Properties: []model.Property{
							{Name: "start", Type: &model.Type{Kind: "reference", Name: "Position"}},
							{Name: "end", Type: &model.Type{Kind: "reference", Name: "Position"}},
						},
					},
				},
			},
			filter: []string{"Range"},
			want:   []string{"Position", "Range"},
		},
		{
			name: "chain A->B->C",
			model: &model.Model{
				Structures: []*model.Structure{
					{
						Name: "A",
						Properties: []model.Property{
							{Name: "b", Type: &model.Type{Kind: "reference", Name: "B"}},
						},
					},
					{
						Name: "B",
						Properties: []model.Property{
							{Name: "c", Type: &model.Type{Kind: "reference", Name: "C"}},
						},
					},
					{
						Name: "C",
						Properties: []model.Property{
							{Name: "value", Type: &model.Type{Kind: "base", Name: "string"}},
						},
					},
				},
			},
			filter: []string{"A"},
			want:   []string{"A", "B", "C"},
		},
		{
			name: "cycle A->B->A",
			model: &model.Model{
				Structures: []*model.Structure{
					{
						Name: "A",
						Properties: []model.Property{
							{Name: "b", Type: &model.Type{Kind: "reference", Name: "B"}},
						},
					},
					{
						Name: "B",
						Properties: []model.Property{
							{Name: "a", Type: &model.Type{Kind: "reference", Name: "A"}},
						},
					},
				},
			},
			filter: []string{"A"},
			want:   []string{"A", "B"},
		},
		{
			name: "array of references",
			model: &model.Model{
				Structures: []*model.Structure{
					{
						Name: "Position",
						Properties: []model.Property{
							{Name: "line", Type: &model.Type{Kind: "base", Name: "uinteger"}},
						},
					},
					{
						Name: "Locations",
						Properties: []model.Property{
							{
								Name: "items",
								Type: &model.Type{
									Kind:    "array",
									Element: &model.Type{Kind: "reference", Name: "Position"},
								},
							},
						},
					},
				},
			},
			filter: []string{"Locations"},
			want:   []string{"Locations", "Position"},
		},
		{
			name: "enum reference",
			model: &model.Model{
				Structures: []*model.Structure{
					{
						Name: "InlayHint",
						Properties: []model.Property{
							{Name: "kind", Type: &model.Type{Kind: "reference", Name: "InlayHintKind"}},
						},
					},
				},
				Enumerations: []*model.Enumeration{
					{
						Name: "InlayHintKind",
						Type: &model.Type{Kind: "base", Name: "uinteger"},
						Values: []model.EnumValue{
							{Name: "Type", Value: float64(1)},
							{Name: "Parameter", Value: float64(2)},
						},
					},
				},
			},
			filter: []string{"InlayHint"},
			want:   []string{"InlayHint", "InlayHintKind"},
		},
		{
			name: "type alias reference",
			model: &model.Model{
				Structures: []*model.Structure{
					{
						Name: "Document",
						Properties: []model.Property{
							{Name: "uri", Type: &model.Type{Kind: "reference", Name: "DocumentUri"}},
						},
					},
				},
				TypeAliases: []*model.TypeAlias{
					{
						Name: "DocumentUri",
						Type: &model.Type{Kind: "base", Name: "string"},
					},
				},
			},
			filter: []string{"Document"},
			want:   []string{"Document", "DocumentUri"},
		},
		{
			name: "type alias chain",
			model: &model.Model{
				TypeAliases: []*model.TypeAlias{
					{
						Name: "OuterAlias",
						Type: &model.Type{Kind: "reference", Name: "InnerAlias"},
					},
					{
						Name: "InnerAlias",
						Type: &model.Type{Kind: "base", Name: "string"},
					},
				},
			},
			filter: []string{"OuterAlias"},
			want:   []string{"InnerAlias", "OuterAlias"},
		},
		{
			name: "or type union",
			model: &model.Model{
				Structures: []*model.Structure{
					{
						Name: "Position",
						Properties: []model.Property{
							{Name: "line", Type: &model.Type{Kind: "base", Name: "uinteger"}},
						},
					},
					{
						Name: "Range",
						Properties: []model.Property{
							{Name: "start", Type: &model.Type{Kind: "reference", Name: "Position"}},
						},
					},
					{
						Name: "Location",
						Properties: []model.Property{
							{
								Name: "target",
								Type: &model.Type{
									Kind: "or",
									Items: []*model.Type{
										{Kind: "reference", Name: "Position"},
										{Kind: "reference", Name: "Range"},
									},
								},
							},
						},
					},
				},
			},
			filter: []string{"Location"},
			want:   []string{"Location", "Position", "Range"},
		},
		{
			name: "map type with reference value",
			model: &model.Model{
				Structures: []*model.Structure{
					{
						Name: "Position",
						Properties: []model.Property{
							{Name: "line", Type: &model.Type{Kind: "base", Name: "uinteger"}},
						},
					},
					{
						Name: "PositionMap",
						Properties: []model.Property{
							{
								Name: "positions",
								Type: &model.Type{
									Kind:  "map",
									Key:   &model.Type{Kind: "base", Name: "string"},
									Value: &model.Type{Kind: "reference", Name: "Position"},
								},
							},
						},
					},
				},
			},
			filter: []string{"PositionMap"},
			want:   []string{"Position", "PositionMap"},
		},
		{
			name: "extends and mixins",
			model: &model.Model{
				Structures: []*model.Structure{
					{
						Name: "Base",
						Properties: []model.Property{
							{Name: "id", Type: &model.Type{Kind: "base", Name: "string"}},
						},
					},
					{
						Name: "Mixin",
						Properties: []model.Property{
							{Name: "extra", Type: &model.Type{Kind: "base", Name: "string"}},
						},
					},
					{
						Name:    "Child",
						Extends: []*model.Type{{Kind: "reference", Name: "Base"}},
						Mixins:  []*model.Type{{Kind: "reference", Name: "Mixin"}},
						Properties: []model.Property{
							{Name: "value", Type: &model.Type{Kind: "base", Name: "string"}},
						},
					},
				},
			},
			filter: []string{"Child"},
			want:   []string{"Base", "Child", "Mixin"},
		},
		{
			name: "base type only no deps",
			model: &model.Model{
				Structures: []*model.Structure{
					{
						Name: "Simple",
						Properties: []model.Property{
							{Name: "value", Type: &model.Type{Kind: "base", Name: "string"}},
						},
					},
				},
			},
			filter: []string{"Simple"},
			want:   []string{"Simple"},
		},
		{
			name: "multiple filter entries",
			model: &model.Model{
				Structures: []*model.Structure{
					{
						Name: "A",
						Properties: []model.Property{
							{Name: "b", Type: &model.Type{Kind: "reference", Name: "B"}},
						},
					},
					{
						Name: "B",
						Properties: []model.Property{
							{Name: "value", Type: &model.Type{Kind: "base", Name: "string"}},
						},
					},
					{
						Name: "C",
						Properties: []model.Property{
							{Name: "value", Type: &model.Type{Kind: "base", Name: "integer"}},
						},
					},
				},
			},
			filter: []string{"A", "C"},
			want:   []string{"A", "B", "C"},
		},
		{
			name: "missing reference is still added to filter",
			model: &model.Model{
				Structures: []*model.Structure{
					{
						Name: "Container",
						Properties: []model.Property{
							{Name: "unknown", Type: &model.Type{Kind: "reference", Name: "Unknown"}},
						},
					},
				},
			},
			filter: []string{"Container"},
			want:   []string{"Container", "Unknown"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Types = tt.filter
			cfg.ResolveDeps = true

			g := New(tt.model, cfg)

			// Call the resolution method
			g.resolveTransitiveDeps()

			// Extract the result
			var got []string
			if g.typeFilter == nil {
				got = nil
			} else {
				for name := range g.typeFilter {
					got = append(got, name)
				}
				sort.Strings(got)
			}

			// Sort want for comparison
			var want []string
			if tt.want != nil {
				want = make([]string, len(tt.want))
				copy(want, tt.want)
				sort.Strings(want)
			}

			if len(got) != len(want) {
				t.Errorf("got %d types, want %d types\ngot:  %v\nwant: %v", len(got), len(want), got, want)
				return
			}

			for i := range got {
				if got[i] != want[i] {
					t.Errorf("type mismatch at index %d\ngot:  %v\nwant: %v", i, got, want)
					return
				}
			}
		})
	}
}

func TestResolveTransitiveDepsDisabled(t *testing.T) {
	m := &model.Model{
		Structures: []*model.Structure{
			{
				Name: "Position",
				Properties: []model.Property{
					{Name: "line", Type: &model.Type{Kind: "base", Name: "uinteger"}},
				},
			},
			{
				Name: "Range",
				Properties: []model.Property{
					{Name: "start", Type: &model.Type{Kind: "reference", Name: "Position"}},
				},
			},
		},
	}

	cfg := DefaultConfig()
	cfg.Types = []string{"Range"}
	cfg.ResolveDeps = false

	g := New(m, cfg)
	g.resolveTransitiveDeps()

	// With ResolveDeps=false, filter should remain unchanged
	if len(g.typeFilter) != 1 {
		t.Errorf("expected 1 type in filter, got %d", len(g.typeFilter))
	}
	if !g.typeFilter["Range"] {
		t.Error("expected Range in filter")
	}
	if g.typeFilter["Position"] {
		t.Error("Position should not be in filter when ResolveDeps=false")
	}
}
