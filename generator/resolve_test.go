// SPDX-License-Identifier: MIT

package generator

import (
	"sort"
	"testing"

	"github.com/albertocavalcante/lspls/model"
)

func TestResolveDeps(t *testing.T) {
	tests := []struct {
		name            string
		model           *model.Model
		filter          map[string]bool
		includeProposed bool
		want            []string // expected types after resolution, nil means nil
	}{
		{
			name: "nil filter returns nil",
			model: &model.Model{
				Structures: []*model.Structure{
					{Name: "Position"},
				},
			},
			filter: nil,
			want:   nil,
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
			filter: map[string]bool{"Range": true},
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
			filter: map[string]bool{"A": true},
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
			filter: map[string]bool{"A": true},
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
			filter: map[string]bool{"Locations": true},
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
			filter: map[string]bool{"InlayHint": true},
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
			filter: map[string]bool{"Document": true},
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
			filter: map[string]bool{"OuterAlias": true},
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
			filter: map[string]bool{"Location": true},
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
			filter: map[string]bool{"PositionMap": true},
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
			filter: map[string]bool{"Child": true},
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
			filter: map[string]bool{"Simple": true},
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
			filter: map[string]bool{"A": true, "C": true},
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
			filter: map[string]bool{"Container": true},
			want:   []string{"Container", "Unknown"},
		},
		{
			name: "proposed property skipped when includeProposed is false",
			model: &model.Model{
				Structures: []*model.Structure{
					{
						Name: "Host",
						Properties: []model.Property{
							{Name: "stable", Type: &model.Type{Kind: "reference", Name: "StableDep"}},
							{Name: "experimental", Proposed: true, Type: &model.Type{Kind: "reference", Name: "ProposedDep"}},
						},
					},
					{
						Name: "StableDep",
						Properties: []model.Property{
							{Name: "v", Type: &model.Type{Kind: "base", Name: "string"}},
						},
					},
					{
						Name: "ProposedDep",
						Properties: []model.Property{
							{Name: "v", Type: &model.Type{Kind: "base", Name: "string"}},
						},
					},
				},
			},
			filter:          map[string]bool{"Host": true},
			includeProposed: false,
			want:            []string{"Host", "StableDep"},
		},
		{
			name: "proposed property included when includeProposed is true",
			model: &model.Model{
				Structures: []*model.Structure{
					{
						Name: "Host",
						Properties: []model.Property{
							{Name: "stable", Type: &model.Type{Kind: "reference", Name: "StableDep"}},
							{Name: "experimental", Proposed: true, Type: &model.Type{Kind: "reference", Name: "ProposedDep"}},
						},
					},
					{
						Name: "StableDep",
						Properties: []model.Property{
							{Name: "v", Type: &model.Type{Kind: "base", Name: "string"}},
						},
					},
					{
						Name: "ProposedDep",
						Properties: []model.Property{
							{Name: "v", Type: &model.Type{Kind: "base", Name: "string"}},
						},
					},
				},
			},
			filter:          map[string]bool{"Host": true},
			includeProposed: true,
			want:            []string{"Host", "ProposedDep", "StableDep"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveDeps(tt.model, tt.filter, tt.includeProposed)

			// Extract result
			var gotSlice []string
			if got == nil {
				gotSlice = nil
			} else {
				for name := range got {
					gotSlice = append(gotSlice, name)
				}
				sort.Strings(gotSlice)
			}

			// Sort want for comparison
			var want []string
			if tt.want != nil {
				want = make([]string, len(tt.want))
				copy(want, tt.want)
				sort.Strings(want)
			}

			if len(gotSlice) != len(want) {
				t.Errorf("got %d types, want %d types\ngot:  %v\nwant: %v", len(gotSlice), len(want), gotSlice, want)
				return
			}

			for i := range gotSlice {
				if gotSlice[i] != want[i] {
					t.Errorf("type mismatch at index %d\ngot:  %v\nwant: %v", i, gotSlice, want)
					return
				}
			}
		})
	}
}
