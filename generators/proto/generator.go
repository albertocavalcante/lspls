// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

package proto

import (
	"context"

	"github.com/albertocavalcante/lspls/generator"
	"github.com/albertocavalcante/lspls/model"
)

// Generator implements [generator.Generator] for Protocol Buffer generation.
type Generator struct{}

// NewGenerator creates a new Proto generator.
func NewGenerator() *Generator {
	return &Generator{}
}

// Metadata returns information about this generator.
func (g *Generator) Metadata() generator.Metadata {
	return generator.Metadata{
		Name:           "proto",
		Version:        "1.0.0",
		Description:    "Generate Protocol Buffer definitions from LSP specification",
		FileExtensions: []string{".proto"},
		URL:            "https://github.com/albertocavalcante/lspls",
	}
}

// Generate produces proto output files from the LSP model.
func (g *Generator) Generate(ctx context.Context, m *model.Model, cfg generator.Config) (*generator.Output, error) {
	// Convert generator.Config to internal Config
	internalCfg := Config{
		PackageName:     cfg.Option("package", "lsp"),
		GoPackage:       cfg.Option("go_package", ""),
		Types:           cfg.Types,
		ResolveDeps:     cfg.ResolveDeps,
		IncludeProposed: cfg.IncludeProposed,
		Source:          cfg.Source,
		Ref:             cfg.Ref,
		CommitHash:      cfg.CommitHash,
		LSPVersion:      cfg.LSPVersion,
	}

	// Create internal generator and generate
	gen := New(m, internalCfg)
	out, err := gen.Generate()
	if err != nil {
		return nil, err
	}

	// Convert to generator.Output
	result := generator.NewOutput()

	// Determine output filename
	filename := "protocol.proto"
	if cfg.OutputFile != "" {
		filename = cfg.OutputFile
	}

	result.Add(filename, out.Proto)
	return result, nil
}
