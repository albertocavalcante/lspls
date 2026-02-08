// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

//go:build e2e

// Package e2e provides end-to-end compile verification tests.
// These tests verify that generated code is valid and compilable.
//
// Run with: go test -tags e2e ./e2e/... -v
// Or:       just test-e2e
package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Tool installation instructions
var installInstructions = map[string]string{
	"go":     "Go is required. Install from https://go.dev/dl/",
	"buf":    "buf is required. Install: go install github.com/bufbuild/buf/cmd/buf@latest",
	"protoc": "protoc is required. Install: https://grpc.io/docs/protoc-installation/",
}

// requireTool fails the test if the tool is not available.
func requireTool(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		instruction := installInstructions[name]
		if instruction == "" {
			instruction = fmt.Sprintf("Install %s and ensure it's in PATH", name)
		}
		t.Fatalf("%s not found in PATH.\n%s", name, instruction)
	}
}

// TestGoOutputCompiles verifies that generated Go code compiles and passes vet.
func TestGoOutputCompiles(t *testing.T) {
	requireTool(t, "go")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Get module root and binary path
	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("find module root: %v", err)
	}

	// Create temp directory for isolated test
	tmpDir := t.TempDir()

	// Build lspls binary with full generators
	binaryPath := filepath.Join(tmpDir, "lspls")
	if err := buildBinaryFull(ctx, moduleRoot, binaryPath); err != nil {
		t.Fatalf("build binary: %v", err)
	}

	// Create isolated Go module
	goModDir := filepath.Join(tmpDir, "gotest")
	if err := os.MkdirAll(goModDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Initialize go.mod
	goModContent := `module lsptest

go 1.22
`
	if err := os.WriteFile(filepath.Join(goModDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	// Generate Go code for a subset of types
	types := "Position,Range,TextEdit,TextDocumentIdentifier"
	outputFile := filepath.Join(goModDir, "protocol.go")

	cmd := exec.CommandContext(ctx, binaryPath,
		"--target=go",
		"-t", types,
		"-o", outputFile,
		"-p", "lsptest",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("lspls generate: %v\n%s", err, stderr.String())
	}

	// Run go build
	t.Run("go_build", func(t *testing.T) {
		start := time.Now()
		cmd := exec.CommandContext(ctx, "go", "build", "./...")
		cmd.Dir = goModDir
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("go build failed: %v\n%s", err, stderr.String())
		}
		t.Logf("go build: %v", time.Since(start))
	})

	// Run go vet
	t.Run("go_vet", func(t *testing.T) {
		start := time.Now()
		cmd := exec.CommandContext(ctx, "go", "vet", "./...")
		cmd.Dir = goModDir
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("go vet failed: %v\n%s", err, stderr.String())
		}
		t.Logf("go vet: %v", time.Since(start))
	})
}

// TestProtoOutputValid verifies that generated proto is valid using buf and protoc.
func TestProtoOutputValid(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Get module root
	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("find module root: %v", err)
	}

	// Create temp directory
	tmpDir := t.TempDir()

	// Build lspls binary with full generators
	binaryPath := filepath.Join(tmpDir, "lspls")
	if err := buildBinaryFull(ctx, moduleRoot, binaryPath); err != nil {
		t.Fatalf("build binary: %v", err)
	}

	// Generate proto for subset of types
	types := "Position,Range,TextEdit,TextDocumentIdentifier,Location"
	protoFile := filepath.Join(tmpDir, "protocol.proto")

	cmd := exec.CommandContext(ctx, binaryPath,
		"--target=proto",
		"-t", types,
		"-o", protoFile,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("lspls generate proto: %v\n%s", err, stderr.String())
	}

	// Read generated proto for validation
	content, err := os.ReadFile(protoFile)
	if err != nil {
		t.Fatalf("read proto: %v", err)
	}

	// Basic structural validation (no external deps)
	t.Run("syntax_check", func(t *testing.T) {
		if !bytes.Contains(content, []byte(`syntax = "proto3";`)) {
			t.Error("missing proto3 syntax declaration")
		}
		if !bytes.Contains(content, []byte("package ")) {
			t.Error("missing package declaration")
		}
		if !bytes.Contains(content, []byte("message ")) {
			t.Error("no message definitions found")
		}
	})

	// buf build (if available) - validates proto structure
	t.Run("buf_build", func(t *testing.T) {
		if _, err := exec.LookPath("buf"); err != nil {
			t.Skip("buf not installed")
		}

		// Create buf.yaml with minimal lint (only structural validation)
		bufYaml := `version: v2
lint:
  use:
    - MINIMAL
`
		if err := os.WriteFile(filepath.Join(tmpDir, "buf.yaml"), []byte(bufYaml), 0644); err != nil {
			t.Fatalf("write buf.yaml: %v", err)
		}

		start := time.Now()
		// Use buf build which validates proto structure without style checks
		cmd := exec.CommandContext(ctx, "buf", "build", protoFile)
		cmd.Dir = tmpDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("buf build output:\n%s", output)
			t.Fatalf("buf build failed: %v", err)
		}
		t.Logf("buf build: %v", time.Since(start))
	})

	// protoc validation (if available)
	t.Run("protoc_validate", func(t *testing.T) {
		if _, err := exec.LookPath("protoc"); err != nil {
			t.Skip("protoc not installed")
		}

		start := time.Now()
		// Use --descriptor_set_out to validate without generating code
		descriptorOut := filepath.Join(tmpDir, "descriptor.pb")
		cmd := exec.CommandContext(ctx, "protoc",
			"--descriptor_set_out="+descriptorOut,
			"-I", tmpDir,
			protoFile,
		)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("protoc output:\n%s", output)
			t.Fatalf("protoc failed: %v", err)
		}
		t.Logf("protoc: %v", time.Since(start))
	})
}

// buildBinaryFull builds lspls with lspls_full tag.
func buildBinaryFull(ctx context.Context, moduleRoot, outputPath string) error {
	cmd := exec.CommandContext(ctx, "go", "build",
		"-tags", "lspls_full",
		"-o", outputPath,
		"./cmd/lspls",
	)
	cmd.Dir = moduleRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build: %w: %s", err, stderr.String())
	}
	return nil
}
