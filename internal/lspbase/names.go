// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

package lspbase

import (
	"strings"
	"unicode"
)

// Capitalize returns name with the first letter uppercased.
// Returns empty string for empty input.
func Capitalize(name string) string {
	if name == "" {
		return ""
	}
	runes := []rune(name)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// StripMeta strips LSP meta-prefixes ("$" and "_") from name.
func StripMeta(name string) string {
	name = strings.TrimPrefix(name, "$")
	name = strings.TrimPrefix(name, "_")
	return name
}

// ExportName returns a Go-safe exported identifier for the given LSP name.
// Names starting with "_" are prefixed with "X" (e.g., "_foo" -> "Xfoo").
// All other names get their first letter uppercased.
func ExportName(name string) string {
	if name == "" {
		return ""
	}
	// Handle names starting with underscore (internal types)
	if name[0] == '_' {
		return "X" + name[1:]
	}
	// Capitalize first letter
	runes := []rune(name)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// CamelToSnake converts a CamelCase name to snake_case.
// Fully uppercase names (like "URI") are lowered as a single word.
func CamelToSnake(name string) string {
	// Check if entire name is uppercase (like URI, ID)
	allUpper := true
	for _, r := range name {
		if !unicode.IsUpper(r) && unicode.IsLetter(r) {
			allUpper = false
			break
		}
	}
	if allUpper {
		return strings.ToLower(name)
	}

	var result strings.Builder
	for i, r := range name {
		if unicode.IsUpper(r) {
			if i > 0 {
				result.WriteRune('_')
			}
			result.WriteRune(unicode.ToLower(r))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// CamelToScreamingSnake converts a CamelCase name to SCREAMING_SNAKE_CASE.
// Fully uppercase names (like "URI") are returned as-is.
func CamelToScreamingSnake(name string) string {
	// Check if entire name is uppercase (like URI, ID)
	allUpper := true
	for _, r := range name {
		if !unicode.IsUpper(r) && unicode.IsLetter(r) {
			allUpper = false
			break
		}
	}
	if allUpper {
		return strings.ToUpper(name)
	}

	var result strings.Builder
	for i, r := range name {
		if unicode.IsUpper(r) && i > 0 {
			result.WriteRune('_')
		}
		result.WriteRune(unicode.ToUpper(r))
	}
	return result.String()
}
