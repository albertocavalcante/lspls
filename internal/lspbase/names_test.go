// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

package lspbase

import "testing"

func TestCapitalize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "lowercase", input: "position", expected: "Position"},
		{name: "already capitalized", input: "Position", expected: "Position"},
		{name: "empty", input: "", expected: ""},
		{name: "single char", input: "a", expected: "A"},
		{name: "all caps", input: "URI", expected: "URI"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Capitalize(tc.input); got != tc.expected {
				t.Errorf("Capitalize(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestStripMeta(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "dollar prefix", input: "$foo", expected: "foo"},
		{name: "underscore prefix", input: "_foo", expected: "foo"},
		{name: "dollar then underscore", input: "$_foo", expected: "foo"},
		{name: "no prefix", input: "foo", expected: "foo"},
		{name: "empty", input: "", expected: ""},
		{name: "only dollar", input: "$", expected: ""},
		{name: "only underscore", input: "_", expected: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := StripMeta(tc.input); got != tc.expected {
				t.Errorf("StripMeta(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestExportName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "already capitalized", input: "Position", expected: "Position"},
		{name: "lowercase first char", input: "position", expected: "Position"},
		{name: "underscore prefix", input: "_internal", expected: "Xinternal"},
		{name: "empty string", input: "", expected: ""},
		{name: "single char", input: "a", expected: "A"},
		{name: "all caps", input: "URL", expected: "URL"},
		{name: "camelCase", input: "textDocument", expected: "TextDocument"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExportName(tc.input); got != tc.expected {
				t.Errorf("ExportName(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestCamelToSnake(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "lowercase", input: "line", expected: "line"},
		{name: "already lowercase", input: "character", expected: "character"},
		{name: "camelCase", input: "textDocument", expected: "text_document"},
		{name: "camelCase with uri", input: "documentUri", expected: "document_uri"},
		{name: "all uppercase", input: "URI", expected: "uri"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := CamelToSnake(tc.input); got != tc.expected {
				t.Errorf("CamelToSnake(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestCamelToScreamingSnake(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "simple", input: "InlayHintKind", expected: "INLAY_HINT_KIND"},
		{name: "single word", input: "Position", expected: "POSITION"},
		{name: "lowercase", input: "position", expected: "POSITION"},
		{name: "two words", input: "TokenFormat", expected: "TOKEN_FORMAT"},
		{name: "already screaming", input: "URI", expected: "URI"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := CamelToScreamingSnake(tc.input); got != tc.expected {
				t.Errorf("CamelToScreamingSnake(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}
