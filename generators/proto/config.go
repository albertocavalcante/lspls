// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

package proto

// Config holds configuration for proto generation.
type Config struct {
	// PackageName is the proto package name (e.g., "lsp").
	PackageName string

	// GoPackage is the go_package option value.
	GoPackage string

	// Types to include (empty means all).
	Types []string

	// ResolveDeps includes transitively referenced types.
	ResolveDeps bool

	// IncludeProposed generates types marked as proposed.
	IncludeProposed bool

	// Source metadata for header comments.
	Source     string
	Ref        string
	CommitHash string
	LSPVersion string

	// TypeOverrides allows custom mapping of LSP types to Proto types.
	// If set, these override DefaultMappings.
	TypeOverrides map[string]string
}

// DefaultMappings provides standard LSP to Proto type mappings.
var DefaultMappings = map[string]string{
	// Simple type aliases -> string
	"DocumentUri":                 "string",
	"URI":                         "string",
	"ChangeAnnotationIdentifier":  "string",
	"Pattern":                     "string",
	"GlobPattern":                 "string",
	"RegularExpressionEngineKind": "string",

	// Progress token can be string or integer - use string for simplicity
	"ProgressToken": "string",

	// Integer-based types
	"DocumentSelector": "string", // Complex type, simplified

	// Dynamic/any types -> google.protobuf.Value
	"LSPAny":    "google.protobuf.Value",
	"LSPObject": "google.protobuf.Struct",
	"LSPArray":  "google.protobuf.ListValue",
}
