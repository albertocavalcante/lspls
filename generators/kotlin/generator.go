// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

package kotlin

import (
	"context"

	"github.com/albertocavalcante/lspls/generator"
	"github.com/albertocavalcante/lspls/model"
)

// Generator implements [generator.Generator] for Kotlin code generation.
type Generator struct{}

// NewGenerator creates a new Kotlin generator.
func NewGenerator() *Generator {
	return &Generator{}
}

// Metadata returns information about this generator.
func (g *Generator) Metadata() generator.Metadata {
	return generator.Metadata{
		Name:           "kotlin",
		Version:        "1.0.0",
		Description:    "Generate Kotlin data classes from LSP specification",
		FileExtensions: []string{".kt"},
		URL:            "https://github.com/albertocavalcante/lspls",
	}
}

// Generate produces Kotlin output files from the LSP model.
func (g *Generator) Generate(ctx context.Context, m *model.Model, cfg generator.Config) (*generator.Output, error) {
	internalCfg := Config{
		PackageName:     cfg.Option("package", "lsp.protocol"),
		Types:           cfg.Types,
		ResolveDeps:     cfg.ResolveDeps,
		IncludeProposed: cfg.IncludeProposed,
		Source:          cfg.Source,
		Ref:             cfg.Ref,
		CommitHash:      cfg.CommitHash,
		LSPVersion:      cfg.LSPVersion,
	}

	gen := New(m, internalCfg)
	out, err := gen.Generate()
	if err != nil {
		return nil, err
	}

	result := generator.NewOutput()

	filename := "Protocol.kt"
	if cfg.OutputFile != "" {
		filename = cfg.OutputFile
	}

	result.Add(filename, out.Kotlin)
	return result, nil
}
