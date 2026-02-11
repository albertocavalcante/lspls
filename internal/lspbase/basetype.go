// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

// Package lspbase provides shared LSP base type classification and name
// transformation utilities used by all code generators.
package lspbase

// LSP base type name constants.
const (
	TypeString      = "string"
	TypeInteger     = "integer"
	TypeUinteger    = "uinteger"
	TypeDecimal     = "decimal"
	TypeBoolean     = "boolean"
	TypeNull        = "null"
	TypeURI         = "URI"
	TypeDocumentURI = "DocumentUri"
	TypeRegExp      = "RegExp"
	TypeLSPAny      = "LSPAny"
	TypeLSPObject   = "LSPObject"
	TypeLSPArray    = "LSPArray"
)

// baseTypes is the set of all recognized LSP base type names.
var baseTypes = map[string]bool{
	TypeString:      true,
	TypeInteger:     true,
	TypeUinteger:    true,
	TypeDecimal:     true,
	TypeBoolean:     true,
	TypeNull:        true,
	TypeURI:         true,
	TypeDocumentURI: true,
	TypeRegExp:      true,
	TypeLSPAny:      true,
	TypeLSPObject:   true,
	TypeLSPArray:    true,
}

// IsBaseType reports whether name is a recognized LSP base type.
func IsBaseType(name string) bool {
	return baseTypes[name]
}

// IsStringLike reports whether the base type maps to a string in most targets.
func IsStringLike(name string) bool {
	switch name {
	case TypeString, TypeURI, TypeDocumentURI, TypeRegExp:
		return true
	}
	return false
}

// IsNumeric reports whether the base type maps to a number in most targets.
func IsNumeric(name string) bool {
	switch name {
	case TypeInteger, TypeUinteger, TypeDecimal:
		return true
	}
	return false
}
