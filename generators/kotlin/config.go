// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

package kotlin

// Config holds configuration for Kotlin generation.
type Config struct {
	// PackageName is the Kotlin package name (e.g., "lsp.protocol").
	PackageName string

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
}

// DefaultMappings provides standard LSP to Kotlin type mappings
// for type aliases that should collapse to a primitive or well-known type.
var DefaultMappings = map[string]string{
	"DocumentUri":                 "String",
	"URI":                         "String",
	"ChangeAnnotationIdentifier":  "String",
	"Pattern":                     "String",
	"GlobPattern":                 "String",
	"RegularExpressionEngineKind": "String",
	"ProgressToken":               "String",
	"DocumentSelector":            "String",
}
