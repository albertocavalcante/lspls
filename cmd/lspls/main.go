// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

// Command lspls generates Go types from the LSP specification.
//
// Usage:
//
//	lspls generate [flags]
//
// Flags:
//
//	-o, --output     Output directory or file (default: stdout)
//	-v, --version    LSP version/git ref (default: 3.17.6)
//	-t, --types      Comma-separated types to generate (default: all)
//	-p, --package    Go package name (default: protocol)
//	--spec           Path to local metaModel.json
//	--repo           Path to local vscode-languageserver-node clone
//	--proposed       Include proposed/unstable features
//	--dry-run        Print to stdout without writing files
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/albertocavalcante/lspls/internal/codegen"
	"github.com/albertocavalcante/lspls/fetch"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Global flags
	showVersion := flag.Bool("version", false, "Show version information")
	showHelp := flag.Bool("help", false, "Show help")

	// Generate command flags
	output := flag.String("o", "", "Output directory or file (default: stdout)")
	lspVersion := flag.String("v", fetch.DefaultRef, "LSP version or git ref")
	types := flag.String("t", "", "Comma-separated types to generate (default: all)")
	packageName := flag.String("p", "protocol", "Go package name")
	specPath := flag.String("spec", "", "Path to local metaModel.json")
	repoDir := flag.String("repo", "", "Path to local vscode-languageserver-node clone")
	proposed := flag.Bool("proposed", false, "Include proposed/unstable features")
	resolveDeps := flag.Bool("resolve-deps", true, "Include transitive type dependencies")
	dryRun := flag.Bool("dry-run", false, "Print to stdout without writing files")
	verbose := flag.Bool("verbose", false, "Verbose output")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `lspls - LSP Protocol Type Generator

Generate Go types from the Language Server Protocol specification.

Usage:
  lspls [flags]

Flags:
  -o string        Output directory or file (default: stdout)
  -v string        LSP version or git ref (default: %s)
  -t string        Comma-separated types to generate (default: all)
  -p string        Go package name (default: protocol)
  --spec string    Path to local metaModel.json
  --repo string    Path to local vscode-languageserver-node clone
  --proposed       Include proposed/unstable features
  --resolve-deps   Include transitive type dependencies (default: true)
  --dry-run        Print to stdout without writing files
  --verbose        Verbose output
  --version        Show version information
  --help           Show this help

Examples:
  # Generate all types to stdout
  lspls

  # Generate to a directory
  lspls -o ./protocol/

  # Generate specific types
  lspls -t InlayHint,InlayHintKind,Position,Range -o ./types.go

  # Use a specific LSP version
  lspls -v release/protocol/3.18.0 -o ./protocol/

  # Use local metaModel.json
  lspls --spec ./metaModel.json -o ./protocol/

`, fetch.DefaultRef)
	}

	flag.Parse()

	if *showHelp {
		flag.Usage()
		return nil
	}

	if *showVersion {
		fmt.Printf("lspls %s (commit: %s, built: %s)\n", version, commit, date)
		return nil
	}

	// Fetch the specification
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if *verbose {
		fmt.Fprintln(os.Stderr, "Fetching LSP specification...")
	}

	fetchOpts := fetch.Options{
		Ref:       *lspVersion,
		LocalPath: *specPath,
		RepoDir:   *repoDir,
		Timeout:   90 * time.Second,
	}

	result, err := fetch.Fetch(ctx, fetchOpts)
	if err != nil {
		return fmt.Errorf("fetch specification: %w", err)
	}

	if *verbose {
		fmt.Fprintf(os.Stderr, "Loaded LSP %s from %s\n", result.Model.Version.Version, result.Source)
		if result.CommitHash != "" {
			fmt.Fprintf(os.Stderr, "Commit: %s\n", result.CommitHash)
		}
		fmt.Fprintf(os.Stderr, "Found %d structures, %d enumerations, %d type aliases\n",
			len(result.Model.Structures),
			len(result.Model.Enumerations),
			len(result.Model.TypeAliases))
	}

	// Configure code generation
	cfg := codegen.Config{
		PackageName:     *packageName,
		ResolveDeps:     *resolveDeps,
		IncludeProposed: *proposed,
		GenerateClient:  true,
		GenerateServer:  true,
		GenerateJSON:    true,
		Source:          result.Source,
		Ref:             result.Ref,
		CommitHash:      result.CommitHash,
		LSPVersion:      result.Model.Version.Version,
	}

	if *types != "" {
		cfg.Types = strings.Split(*types, ",")
		for i := range cfg.Types {
			cfg.Types[i] = strings.TrimSpace(cfg.Types[i])
		}
	}

	// Generate code
	gen := codegen.New(result.Model, cfg)
	out, err := gen.Generate()
	if err != nil {
		return fmt.Errorf("generate code: %w", err)
	}

	// Output
	if *dryRun || *output == "" {
		fmt.Println(string(out.Protocol))
		return nil
	}

	// Write to file or directory
	outputPath := *output
	if strings.HasSuffix(outputPath, "/") || isDir(outputPath) {
		// Directory output - write multiple files
		if err := os.MkdirAll(outputPath, 0o755); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}

		if err := os.WriteFile(filepath.Join(outputPath, "protocol.go"), out.Protocol, 0o644); err != nil {
			return fmt.Errorf("write protocol.go: %w", err)
		}

		if *verbose {
			fmt.Fprintf(os.Stderr, "Wrote %s/protocol.go\n", outputPath)
		}
	} else {
		// Single file output
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}

		if err := os.WriteFile(outputPath, out.Protocol, 0o644); err != nil {
			return fmt.Errorf("write output: %w", err)
		}

		if *verbose {
			fmt.Fprintf(os.Stderr, "Wrote %s\n", outputPath)
		}
	}

	return nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
