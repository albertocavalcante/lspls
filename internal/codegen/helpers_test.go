// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

package codegen

import (
	"testing"

	"github.com/albertocavalcante/lspls/internal/model"
)

func TestExportName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "already capitalized",
			input:    "Position",
			expected: "Position",
		},
		{
			name:     "lowercase first char",
			input:    "position",
			expected: "Position",
		},
		{
			name:     "underscore prefix",
			input:    "_internal",
			expected: "Xinternal",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "single char",
			input:    "a",
			expected: "A",
		},
		{
			name:     "all caps",
			input:    "URL",
			expected: "URL",
		},
		{
			name:     "camelCase",
			input:    "textDocument",
			expected: "TextDocument",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := exportName(tc.input)
			if result != tc.expected {
				t.Errorf("exportName(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestFormatConstValue(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		baseType string
		expected string
	}{
		{
			name:     "string value",
			value:    "foo",
			baseType: "string",
			expected: `"foo"`,
		},
		{
			name:     "integer value",
			value:    42.0,
			baseType: "integer",
			expected: "42",
		},
		{
			name:     "decimal truncated",
			value:    3.14,
			baseType: "decimal",
			expected: "3",
		},
		{
			name:     "non-numeric as string",
			value:    "bar",
			baseType: "integer",
			expected: `"bar"`,
		},
		{
			name:     "float64 with string baseType",
			value:    123.0,
			baseType: "string",
			expected: `"123"`,
		},
		{
			name:     "zero integer",
			value:    0.0,
			baseType: "integer",
			expected: "0",
		},
		{
			name:     "negative integer",
			value:    -5.0,
			baseType: "integer",
			expected: "-5",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := formatConstValue(tc.value, tc.baseType)
			if result != tc.expected {
				t.Errorf("formatConstValue(%v, %q) = %q, want %q", tc.value, tc.baseType, result, tc.expected)
			}
		})
	}
}

func TestGoBaseType(t *testing.T) {
	// Create a minimal generator for testing
	g := New(&model.Model{}, DefaultConfig())

	tests := []struct {
		name     string
		input    *model.Type
		expected string
	}{
		{
			name:     "string",
			input:    &model.Type{Kind: "base", Name: "string"},
			expected: "string",
		},
		{
			name:     "integer",
			input:    &model.Type{Kind: "base", Name: "integer"},
			expected: "int32",
		},
		{
			name:     "uinteger",
			input:    &model.Type{Kind: "base", Name: "uinteger"},
			expected: "uint32",
		},
		{
			name:     "decimal",
			input:    &model.Type{Kind: "base", Name: "decimal"},
			expected: "float64",
		},
		{
			name:     "boolean",
			input:    &model.Type{Kind: "base", Name: "boolean"},
			expected: "bool",
		},
		{
			name:     "null",
			input:    &model.Type{Kind: "base", Name: "null"},
			expected: "any",
		},
		{
			name:     "DocumentUri",
			input:    &model.Type{Kind: "base", Name: "DocumentUri"},
			expected: "string",
		},
		{
			name:     "URI",
			input:    &model.Type{Kind: "base", Name: "URI"},
			expected: "string",
		},
		{
			name:     "LSPAny",
			input:    &model.Type{Kind: "base", Name: "LSPAny"},
			expected: "any",
		},
		{
			name:     "unknown base type",
			input:    &model.Type{Kind: "base", Name: "unknown"},
			expected: "any",
		},
		{
			name:     "nil type",
			input:    nil,
			expected: "any",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := g.goBaseType(tc.input)
			if result != tc.expected {
				t.Errorf("goBaseType(%+v) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestTypeNameForIdent(t *testing.T) {
	// Create a minimal generator for testing
	g := New(&model.Model{}, DefaultConfig())

	tests := []struct {
		name     string
		input    *model.Type
		expected string
	}{
		{
			name:     "base type string",
			input:    &model.Type{Kind: "base", Name: "string"},
			expected: "string",
		},
		{
			name:     "base type integer",
			input:    &model.Type{Kind: "base", Name: "integer"},
			expected: "int32",
		},
		{
			name:     "reference",
			input:    &model.Type{Kind: "reference", Name: "Position"},
			expected: "Position",
		},
		{
			name:     "reference lowercase",
			input:    &model.Type{Kind: "reference", Name: "position"},
			expected: "Position",
		},
		{
			name: "array of reference",
			input: &model.Type{
				Kind:    "array",
				Element: &model.Type{Kind: "reference", Name: "Location"},
			},
			expected: "ArrLocation",
		},
		{
			name: "nested array",
			input: &model.Type{
				Kind: "array",
				Element: &model.Type{
					Kind:    "array",
					Element: &model.Type{Kind: "reference", Name: "Diagnostic"},
				},
			},
			expected: "ArrArrDiagnostic",
		},
		{
			name: "map type",
			input: &model.Type{
				Kind:  "map",
				Key:   &model.Type{Kind: "base", Name: "string"},
				Value: &model.Type{Kind: "reference", Name: "TextEdit"},
			},
			expected: "MapstringTextEdit",
		},
		{
			name:     "literal type",
			input:    &model.Type{Kind: "literal"},
			expected: "Literal",
		},
		{
			name:     "stringLiteral type",
			input:    &model.Type{Kind: "stringLiteral", Value: "create"},
			expected: "string",
		},
		{
			name:     "or type",
			input:    &model.Type{Kind: "or"},
			expected: "Union",
		},
		{
			name:     "and type",
			input:    &model.Type{Kind: "and"},
			expected: "Intersection",
		},
		{
			name:     "tuple type",
			input:    &model.Type{Kind: "tuple"},
			expected: "Tuple",
		},
		{
			name:     "nil type",
			input:    nil,
			expected: "any",
		},
		{
			name:     "unknown kind",
			input:    &model.Type{Kind: "unknown"},
			expected: "any",
		},
		{
			name: "map with nil value",
			input: &model.Type{
				Kind:  "map",
				Key:   &model.Type{Kind: "base", Name: "string"},
				Value: nil, // Value is not a *model.Type
			},
			expected: "Mapstringany",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := g.typeNameForIdent(tc.input)
			if result != tc.expected {
				t.Errorf("typeNameForIdent(%+v) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}
