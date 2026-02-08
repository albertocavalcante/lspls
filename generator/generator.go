// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

// Package generator defines the interface for LSP code generators.
package generator

import (
	"context"

	"github.com/albertocavalcante/lspls/model"
)

// Generator is the interface that all code generators must implement.
type Generator interface {
	// Metadata returns information about this generator.
	Metadata() Metadata

	// Generate produces output files from the LSP model.
	Generate(ctx context.Context, m *model.Model, cfg Config) (*Output, error)
}

// Metadata describes a generator.
type Metadata struct {
	// Name is the short identifier (e.g., "go", "proto", "thrift").
	Name string

	// Version is the generator version (semver).
	Version string

	// Description is a human-readable description.
	Description string

	// FileExtensions lists typical output extensions (e.g., [".go"], [".proto"]).
	FileExtensions []string

	// URL is the homepage/documentation URL (optional).
	URL string
}
