// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

package lspbase

import "testing"

func TestIsBaseType(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{name: "string", want: true},
		{name: "integer", want: true},
		{name: "uinteger", want: true},
		{name: "decimal", want: true},
		{name: "boolean", want: true},
		{name: "null", want: true},
		{name: "URI", want: true},
		{name: "DocumentUri", want: true},
		{name: "RegExp", want: true},
		{name: "LSPAny", want: true},
		{name: "LSPObject", want: true},
		{name: "LSPArray", want: true},
		{name: "unknown", want: false},
		{name: "", want: false},
		{name: "String", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsBaseType(tt.name); got != tt.want {
				t.Errorf("IsBaseType(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestIsStringLike(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{name: "string", want: true},
		{name: "URI", want: true},
		{name: "DocumentUri", want: true},
		{name: "RegExp", want: true},
		{name: "integer", want: false},
		{name: "boolean", want: false},
		{name: "null", want: false},
		{name: "LSPAny", want: false},
		{name: "unknown", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsStringLike(tt.name); got != tt.want {
				t.Errorf("IsStringLike(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{name: "integer", want: true},
		{name: "uinteger", want: true},
		{name: "decimal", want: true},
		{name: "string", want: false},
		{name: "boolean", want: false},
		{name: "null", want: false},
		{name: "URI", want: false},
		{name: "unknown", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNumeric(tt.name); got != tt.want {
				t.Errorf("IsNumeric(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
