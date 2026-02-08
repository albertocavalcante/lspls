// SPDX-License-Identifier: MIT AND BSD-3-Clause
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.

package model

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestType_IsOptional(t *testing.T) {
	tests := []struct {
		name     string
		typ      *Type
		expected bool
	}{
		{
			name: "or type with string and null - null second",
			typ: &Type{
				Kind: "or",
				Items: []*Type{
					{Kind: "base", Name: "string"},
					{Kind: "base", Name: "null"},
				},
			},
			expected: true,
		},
		{
			name: "or type with null and string - null first",
			typ: &Type{
				Kind: "or",
				Items: []*Type{
					{Kind: "base", Name: "null"},
					{Kind: "base", Name: "string"},
				},
			},
			expected: true,
		},
		{
			name: "or type with string and number - no null",
			typ: &Type{
				Kind: "or",
				Items: []*Type{
					{Kind: "base", Name: "string"},
					{Kind: "base", Name: "number"},
				},
			},
			expected: false,
		},
		{
			name: "or type with more than 2 items including null",
			typ: &Type{
				Kind: "or",
				Items: []*Type{
					{Kind: "base", Name: "string"},
					{Kind: "base", Name: "number"},
					{Kind: "base", Name: "null"},
				},
			},
			expected: false,
		},
		{
			name: "base type - not an or type",
			typ: &Type{
				Kind: "base",
				Name: "string",
			},
			expected: false,
		},
		{
			name: "or type with single null item",
			typ: &Type{
				Kind: "or",
				Items: []*Type{
					{Kind: "base", Name: "null"},
				},
			},
			expected: false,
		},
		{
			name: "or type with empty items",
			typ: &Type{
				Kind:  "or",
				Items: []*Type{},
			},
			expected: false,
		},
		{
			name: "reference type - not an or type",
			typ: &Type{
				Kind: "reference",
				Name: "Position",
			},
			expected: false,
		},
		{
			name: "or type with reference and null",
			typ: &Type{
				Kind: "or",
				Items: []*Type{
					{Kind: "reference", Name: "Position"},
					{Kind: "base", Name: "null"},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.typ.IsOptional()
			if got != tt.expected {
				t.Errorf("IsOptional() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestType_NonNullType(t *testing.T) {
	tests := []struct {
		name     string
		typ      *Type
		expected *Type
	}{
		{
			name: "optional type returns non-null component - null second",
			typ: &Type{
				Kind: "or",
				Items: []*Type{
					{Kind: "base", Name: "string"},
					{Kind: "base", Name: "null"},
				},
			},
			expected: &Type{Kind: "base", Name: "string"},
		},
		{
			name: "optional type returns non-null component - null first",
			typ: &Type{
				Kind: "or",
				Items: []*Type{
					{Kind: "base", Name: "null"},
					{Kind: "base", Name: "string"},
				},
			},
			expected: &Type{Kind: "base", Name: "string"},
		},
		{
			name: "optional reference type",
			typ: &Type{
				Kind: "or",
				Items: []*Type{
					{Kind: "reference", Name: "Position"},
					{Kind: "base", Name: "null"},
				},
			},
			expected: &Type{Kind: "reference", Name: "Position"},
		},
		{
			name: "non-optional type returns nil",
			typ: &Type{
				Kind: "base",
				Name: "string",
			},
			expected: nil,
		},
		{
			name: "or type without null returns nil",
			typ: &Type{
				Kind: "or",
				Items: []*Type{
					{Kind: "base", Name: "string"},
					{Kind: "base", Name: "number"},
				},
			},
			expected: nil,
		},
		{
			name: "or type with more than 2 items returns nil",
			typ: &Type{
				Kind: "or",
				Items: []*Type{
					{Kind: "base", Name: "string"},
					{Kind: "base", Name: "number"},
					{Kind: "base", Name: "null"},
				},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.typ.NonNullType()
			if tt.expected == nil {
				if got != nil {
					t.Errorf("NonNullType() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Errorf("NonNullType() = nil, want %v", tt.expected)
				return
			}
			if got.Kind != tt.expected.Kind || got.Name != tt.expected.Name {
				t.Errorf("NonNullType() = {Kind: %q, Name: %q}, want {Kind: %q, Name: %q}",
					got.Kind, got.Name, tt.expected.Kind, tt.expected.Name)
			}
		})
	}
}

func TestType_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    *Type
		expectError bool
	}{
		{
			name:  "base type string",
			input: `{"kind":"base","name":"string"}`,
			expected: &Type{
				Kind: "base",
				Name: "string",
			},
			expectError: false,
		},
		{
			name:  "base type integer",
			input: `{"kind":"base","name":"integer"}`,
			expected: &Type{
				Kind: "base",
				Name: "integer",
			},
			expectError: false,
		},
		{
			name:  "reference type",
			input: `{"kind":"reference","name":"Position"}`,
			expected: &Type{
				Kind: "reference",
				Name: "Position",
			},
			expectError: false,
		},
		{
			name:  "array type",
			input: `{"kind":"array","element":{"kind":"base","name":"string"}}`,
			expected: &Type{
				Kind:    "array",
				Element: &Type{Kind: "base", Name: "string"},
			},
			expectError: false,
		},
		{
			name:  "nested array type",
			input: `{"kind":"array","element":{"kind":"array","element":{"kind":"base","name":"integer"}}}`,
			expected: &Type{
				Kind: "array",
				Element: &Type{
					Kind:    "array",
					Element: &Type{Kind: "base", Name: "integer"},
				},
			},
			expectError: false,
		},
		{
			name:  "map type",
			input: `{"kind":"map","key":{"kind":"base","name":"string"},"value":{"kind":"base","name":"integer"}}`,
			expected: &Type{
				Kind:  "map",
				Key:   &Type{Kind: "base", Name: "string"},
				Value: &Type{Kind: "base", Name: "integer"},
			},
			expectError: false,
		},
		{
			name:  "or type with two items",
			input: `{"kind":"or","items":[{"kind":"base","name":"string"},{"kind":"base","name":"null"}]}`,
			expected: &Type{
				Kind: "or",
				Items: []*Type{
					{Kind: "base", Name: "string"},
					{Kind: "base", Name: "null"},
				},
			},
			expectError: false,
		},
		{
			name:  "or type with three items",
			input: `{"kind":"or","items":[{"kind":"base","name":"string"},{"kind":"base","name":"integer"},{"kind":"base","name":"boolean"}]}`,
			expected: &Type{
				Kind: "or",
				Items: []*Type{
					{Kind: "base", Name: "string"},
					{Kind: "base", Name: "integer"},
					{Kind: "base", Name: "boolean"},
				},
			},
			expectError: false,
		},
		{
			name:  "and type",
			input: `{"kind":"and","items":[{"kind":"reference","name":"TextDocumentIdentifier"},{"kind":"reference","name":"Position"}]}`,
			expected: &Type{
				Kind: "and",
				Items: []*Type{
					{Kind: "reference", Name: "TextDocumentIdentifier"},
					{Kind: "reference", Name: "Position"},
				},
			},
			expectError: false,
		},
		{
			name:  "tuple type",
			input: `{"kind":"tuple","items":[{"kind":"base","name":"integer"},{"kind":"base","name":"integer"}]}`,
			expected: &Type{
				Kind: "tuple",
				Items: []*Type{
					{Kind: "base", Name: "integer"},
					{Kind: "base", Name: "integer"},
				},
			},
			expectError: false,
		},
		{
			name:  "stringLiteral type",
			input: `{"kind":"stringLiteral","value":"utf-8"}`,
			expected: &Type{
				Kind:  "stringLiteral",
				Value: "utf-8",
			},
			expectError: false,
		},
		{
			name:  "literal type with properties",
			input: `{"kind":"literal","value":{"properties":[{"name":"line","type":{"kind":"base","name":"uinteger"}},{"name":"character","type":{"kind":"base","name":"uinteger"}}]}}`,
			expected: &Type{
				Kind: "literal",
				Value: Literal{
					Properties: []Property{
						{Name: "line", Type: &Type{Kind: "base", Name: "uinteger"}},
						{Name: "character", Type: &Type{Kind: "base", Name: "uinteger"}},
					},
				},
			},
			expectError: false,
		},
		{
			name:  "literal type with empty properties",
			input: `{"kind":"literal","value":{"properties":[]}}`,
			expected: &Type{
				Kind: "literal",
				Value: Literal{
					Properties: []Property{},
				},
			},
			expectError: false,
		},
		{
			name:  "type with line number",
			input: `{"kind":"base","name":"string","line":42}`,
			expected: &Type{
				Kind: "base",
				Name: "string",
				Line: 42,
			},
			expectError: false,
		},
		{
			name:        "invalid JSON",
			input:       `{"kind":"base","name":}`,
			expected:    nil,
			expectError: true,
		},
		{
			name:        "unknown kind",
			input:       `{"kind":"unknown","name":"test"}`,
			expected:    nil,
			expectError: true,
		},
		{
			name:        "empty JSON object",
			input:       `{}`,
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Type
			err := json.Unmarshal([]byte(tt.input), &got)

			if tt.expectError {
				if err == nil {
					t.Errorf("UnmarshalJSON() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("UnmarshalJSON() unexpected error: %v", err)
				return
			}

			if !typeEqual(&got, tt.expected) {
				t.Errorf("UnmarshalJSON() = %+v, want %+v", got, tt.expected)
			}
		})
	}
}

func TestType_UnmarshalJSON_MapValue(t *testing.T) {
	// Test that map type correctly unmarshals value as *Type
	input := `{"kind":"map","key":{"kind":"base","name":"string"},"value":{"kind":"reference","name":"TextEdit"}}`

	var got Type
	err := json.Unmarshal([]byte(input), &got)
	if err != nil {
		t.Fatalf("UnmarshalJSON() unexpected error: %v", err)
	}

	if got.Kind != "map" {
		t.Errorf("Kind = %q, want %q", got.Kind, "map")
	}
	if got.Key == nil {
		t.Error("Key is nil")
	} else if got.Key.Kind != "base" || got.Key.Name != "string" {
		t.Errorf("Key = %+v, want {Kind: base, Name: string}", got.Key)
	}

	valueType, ok := got.Value.(*Type)
	if !ok {
		t.Fatalf("Value is not *Type, got %T", got.Value)
	}
	if valueType.Kind != "reference" || valueType.Name != "TextEdit" {
		t.Errorf("Value = %+v, want {Kind: reference, Name: TextEdit}", valueType)
	}
}

func TestType_UnmarshalJSON_LiteralValue(t *testing.T) {
	// Test that literal type correctly unmarshals value as Literal
	input := `{"kind":"literal","value":{"properties":[{"name":"uri","type":{"kind":"base","name":"DocumentUri"},"optional":true,"documentation":"The URI of the document."}]}}`

	var got Type
	err := json.Unmarshal([]byte(input), &got)
	if err != nil {
		t.Fatalf("UnmarshalJSON() unexpected error: %v", err)
	}

	if got.Kind != "literal" {
		t.Errorf("Kind = %q, want %q", got.Kind, "literal")
	}

	literal, ok := got.Value.(Literal)
	if !ok {
		t.Fatalf("Value is not Literal, got %T", got.Value)
	}

	if len(literal.Properties) != 1 {
		t.Fatalf("Properties length = %d, want 1", len(literal.Properties))
	}

	prop := literal.Properties[0]
	if prop.Name != "uri" {
		t.Errorf("Property Name = %q, want %q", prop.Name, "uri")
	}
	if !prop.Optional {
		t.Error("Property Optional = false, want true")
	}
	if prop.Documentation != "The URI of the document." {
		t.Errorf("Property Documentation = %q, want %q", prop.Documentation, "The URI of the document.")
	}
	if prop.Type == nil {
		t.Error("Property Type is nil")
	} else if prop.Type.Kind != "base" || prop.Type.Name != "DocumentUri" {
		t.Errorf("Property Type = %+v, want {Kind: base, Name: DocumentUri}", prop.Type)
	}
}

func TestModel_UnmarshalJSON(t *testing.T) {
	input := `{
		"metaData": {"version": "3.17.0"},
		"requests": [
			{
				"method": "textDocument/hover",
				"messageDirection": "clientToServer",
				"params": {"kind": "reference", "name": "HoverParams"},
				"result": {"kind": "or", "items": [{"kind": "reference", "name": "Hover"}, {"kind": "base", "name": "null"}]}
			}
		],
		"notifications": [
			{
				"method": "initialized",
				"messageDirection": "clientToServer"
			}
		],
		"structures": [
			{
				"name": "Position",
				"properties": [
					{"name": "line", "type": {"kind": "base", "name": "uinteger"}},
					{"name": "character", "type": {"kind": "base", "name": "uinteger"}}
				]
			}
		],
		"enumerations": [
			{
				"name": "DiagnosticSeverity",
				"type": {"kind": "base", "name": "uinteger"},
				"values": [
					{"name": "Error", "value": 1},
					{"name": "Warning", "value": 2}
				]
			}
		],
		"typeAliases": [
			{
				"name": "DocumentUri",
				"type": {"kind": "base", "name": "string"}
			}
		]
	}`

	var model Model
	err := json.Unmarshal([]byte(input), &model)
	if err != nil {
		t.Fatalf("UnmarshalJSON() unexpected error: %v", err)
	}

	// Verify metadata
	if model.Version.Version != "3.17.0" {
		t.Errorf("Version = %q, want %q", model.Version.Version, "3.17.0")
	}

	// Verify requests
	if len(model.Requests) != 1 {
		t.Fatalf("Requests length = %d, want 1", len(model.Requests))
	}
	req := model.Requests[0]
	if req.Method != "textDocument/hover" {
		t.Errorf("Request Method = %q, want %q", req.Method, "textDocument/hover")
	}
	if req.Direction != "clientToServer" {
		t.Errorf("Request Direction = %q, want %q", req.Direction, "clientToServer")
	}
	if req.Params == nil || req.Params.Kind != "reference" || req.Params.Name != "HoverParams" {
		t.Errorf("Request Params = %+v, want reference to HoverParams", req.Params)
	}
	if req.Result == nil || req.Result.Kind != "or" || len(req.Result.Items) != 2 {
		t.Errorf("Request Result = %+v, want or type with 2 items", req.Result)
	}

	// Verify notifications
	if len(model.Notifications) != 1 {
		t.Fatalf("Notifications length = %d, want 1", len(model.Notifications))
	}
	notif := model.Notifications[0]
	if notif.Method != "initialized" {
		t.Errorf("Notification Method = %q, want %q", notif.Method, "initialized")
	}

	// Verify structures
	if len(model.Structures) != 1 {
		t.Fatalf("Structures length = %d, want 1", len(model.Structures))
	}
	structure := model.Structures[0]
	if structure.Name != "Position" {
		t.Errorf("Structure Name = %q, want %q", structure.Name, "Position")
	}
	if len(structure.Properties) != 2 {
		t.Fatalf("Structure Properties length = %d, want 2", len(structure.Properties))
	}

	// Verify enumerations
	if len(model.Enumerations) != 1 {
		t.Fatalf("Enumerations length = %d, want 1", len(model.Enumerations))
	}
	enum := model.Enumerations[0]
	if enum.Name != "DiagnosticSeverity" {
		t.Errorf("Enumeration Name = %q, want %q", enum.Name, "DiagnosticSeverity")
	}
	if len(enum.Values) != 2 {
		t.Fatalf("Enumeration Values length = %d, want 2", len(enum.Values))
	}

	// Verify type aliases
	if len(model.TypeAliases) != 1 {
		t.Fatalf("TypeAliases length = %d, want 1", len(model.TypeAliases))
	}
	alias := model.TypeAliases[0]
	if alias.Name != "DocumentUri" {
		t.Errorf("TypeAlias Name = %q, want %q", alias.Name, "DocumentUri")
	}
}

// typeEqual compares two Type structs for equality
func typeEqual(a, b *Type) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	if a.Kind != b.Kind || a.Name != b.Name || a.Line != b.Line {
		return false
	}

	// Compare Items
	if len(a.Items) != len(b.Items) {
		return false
	}
	for i := range a.Items {
		if !typeEqual(a.Items[i], b.Items[i]) {
			return false
		}
	}

	// Compare Element
	if !typeEqual(a.Element, b.Element) {
		return false
	}

	// Compare Key
	if !typeEqual(a.Key, b.Key) {
		return false
	}

	// Compare Value (special handling for different types)
	switch av := a.Value.(type) {
	case nil:
		if b.Value != nil {
			return false
		}
	case string:
		bv, ok := b.Value.(string)
		if !ok || av != bv {
			return false
		}
	case *Type:
		bv, ok := b.Value.(*Type)
		if !ok || !typeEqual(av, bv) {
			return false
		}
	case Literal:
		bv, ok := b.Value.(Literal)
		if !ok || !reflect.DeepEqual(av, bv) {
			return false
		}
	default:
		// For other types, use reflect.DeepEqual
		if !reflect.DeepEqual(a.Value, b.Value) {
			return false
		}
	}

	return true
}
