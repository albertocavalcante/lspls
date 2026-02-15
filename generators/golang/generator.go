// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

package golang

import (
	"context"

	"github.com/albertocavalcante/lspls/generator"
	"github.com/albertocavalcante/lspls/model"
)

// GoGenerator implements [generator.Generator] for Go code generation.
type GoGenerator struct{}

// New creates a new Go generator.
func NewGenerator() *GoGenerator {
	return &GoGenerator{}
}

// Metadata returns information about this generator.
func (g *GoGenerator) Metadata() generator.Metadata {
	return generator.Metadata{
		Name:           "go",
		Version:        "1.0.0",
		Description:    "Generate Go types from LSP specification",
		FileExtensions: []string{".go"},
		URL:            "https://github.com/albertocavalcante/lspls",
	}
}

// Generate produces Go output files from the LSP model.
func (g *GoGenerator) Generate(ctx context.Context, m *model.Model, cfg generator.Config) (*generator.Output, error) {
	// Convert generator.Config to internal Config
	internalCfg := Config{
		PackageName:     cfg.Option("package", "protocol"),
		Types:           cfg.Types,
		ResolveDeps:     cfg.ResolveDeps,
		IncludeProposed: cfg.IncludeProposed,
		GenerateClient:  cfg.GenerateClient,
		GenerateServer:  cfg.GenerateServer,
		GenerateJSON:    true,
		Source:          cfg.Source,
		Ref:             cfg.Ref,
		CommitHash:      cfg.CommitHash,
		LSPVersion:      cfg.LSPVersion,
	}

	// Enable split files when writing to a directory
	if cfg.OutputDir != "" {
		internalCfg.SplitFiles = true
	}

	// Create internal generator and generate
	gen := New(m, internalCfg)
	out, err := gen.Generate()
	if err != nil {
		return nil, err
	}

	// Convert to generator.Output
	result := generator.NewOutput()

	// Determine output filename for protocol types
	filename := "protocol.go"
	if cfg.OutputFile != "" {
		filename = cfg.OutputFile
	}

	result.Add(filename, out.Protocol)
	if out.Server != nil {
		result.Add("server.go", out.Server)
	}
	if out.Client != nil {
		result.Add("client.go", out.Client)
	}
	if out.JSON != nil {
		result.Add("json.go", out.JSON)
	}
	return result, nil
}
