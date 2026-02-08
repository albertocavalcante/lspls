// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

package generator

// Config contains generator configuration.
type Config struct {
	// OutputDir is the output directory.
	OutputDir string

	// OutputFile is for single file output (optional).
	OutputFile string

	// Types filters to specific type names (empty = all).
	Types []string

	// ResolveDeps includes transitive dependencies when filtering.
	ResolveDeps bool

	// IncludeProposed includes @proposed features.
	IncludeProposed bool

	// GenerateClient generates client interface.
	GenerateClient bool

	// GenerateServer generates server interface.
	GenerateServer bool

	// Source is the spec source (for headers).
	Source string

	// Ref is the git ref used.
	Ref string

	// CommitHash is the git commit.
	CommitHash string

	// LSPVersion is the protocol version.
	LSPVersion string

	// Options contains target-specific options.
	Options map[string]string
}

// Option returns a target-specific option with default.
func (c Config) Option(key, defaultValue string) string {
	if v, ok := c.Options[key]; ok {
		return v
	}
	return defaultValue
}
