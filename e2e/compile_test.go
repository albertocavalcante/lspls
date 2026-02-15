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
	"strings"
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
	types := "Position,Range,TextEdit,TextDocumentIdentifier,Location,WorkspaceEdit"
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

// TestGroovyOutputCompiles verifies that generated Groovy code compiles and
// passes Jackson serialization smoke tests via Gradle.
func TestGroovyOutputCompiles(t *testing.T) {
	if _, err := exec.LookPath("gradle"); err != nil {
		t.Skip("gradle not installed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("find module root: %v", err)
	}

	tmpDir := t.TempDir()

	// Build lspls binary with full generators
	binaryPath := filepath.Join(tmpDir, "lspls")
	if err := buildBinaryFull(ctx, moduleRoot, binaryPath); err != nil {
		t.Fatalf("build binary: %v", err)
	}

	// Set up a Gradle project in the temp directory by copying the example scaffolding
	exampleDir := filepath.Join(moduleRoot, "examples", "groovy-lsp")
	for _, name := range []string{"build.gradle", "settings.gradle"} {
		data, err := os.ReadFile(filepath.Join(exampleDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, name), data, 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	// Create source directories
	srcDir := filepath.Join(tmpDir, "src", "main", "groovy", "lsp", "protocol")
	testDir := filepath.Join(tmpDir, "src", "test", "groovy", "lsp", "protocol")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("mkdir test: %v", err)
	}

	// Generate Groovy code for a subset of types
	types := "Position,Range,TextEdit,TextDocumentIdentifier,DiagnosticSeverity,MarkupKind"
	outputFile := filepath.Join(srcDir, "Protocol.groovy")

	cmd := exec.CommandContext(ctx, binaryPath,
		"--target=groovy",
		"-t", types,
		"-p", "lsp.protocol",
		"-o", outputFile,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("lspls generate groovy: %v\n%s", err, stderr.String())
	}

	// Copy the smoke test from the example
	testSrc := filepath.Join(exampleDir, "src", "test", "groovy", "lsp", "protocol", "ProtocolSmokeTest.groovy")
	testData, err := os.ReadFile(testSrc)
	if err != nil {
		t.Fatalf("read smoke test: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testDir, "ProtocolSmokeTest.groovy"), testData, 0644); err != nil {
		t.Fatalf("write smoke test: %v", err)
	}

	// Run gradle test
	t.Run("gradle_test", func(t *testing.T) {
		start := time.Now()
		cmd := exec.CommandContext(ctx, "gradle", "test", "--no-daemon")
		cmd.Dir = tmpDir
		// Ensure JAVA_HOME is set — some environments (e.g. sdkman)
		// only set it inside interactive shells.
		env := ensureJavaHome(os.Environ())
		// Ensure GRADLE_USER_HOME points to the default cache so the temp
		// project reuses already-downloaded dependencies instead of
		// downloading everything from scratch (which would exceed the timeout).
		env = ensureGradleHome(env)
		cmd.Env = env
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("gradle output:\n%s", output)
			t.Fatalf("gradle test failed: %v", err)
		}
		t.Logf("gradle test: %v", time.Since(start))
	})
}

// ensureJavaHome returns env with a valid JAVA_HOME. If the existing value
// points to a valid JDK directory it is kept. Otherwise JAVA_HOME is resolved
// from well-known locations (sdkman, Homebrew, Gradle-provisioned JDKs).
func ensureJavaHome(env []string) []string {
	// Check if existing JAVA_HOME is valid (must have bin/java AND lib or release file).
	for _, e := range env {
		if len(e) > 10 && e[:10] == "JAVA_HOME=" {
			if isValidJDK(e[10:]) {
				return env
			}
		}
	}

	// Try well-known JDK locations in order of preference.
	home := findJDKHome()
	if home == "" {
		return env
	}

	// Replace any existing invalid JAVA_HOME entry.
	replaced := false
	for i, e := range env {
		if len(e) > 10 && e[:10] == "JAVA_HOME=" {
			env[i] = "JAVA_HOME=" + home
			replaced = true
		}
	}
	if !replaced {
		env = append(env, "JAVA_HOME="+home)
	}
	return env
}

// isValidJDK checks that a directory looks like a real JDK (not the macOS stub).
func isValidJDK(home string) bool {
	info, err := os.Stat(filepath.Join(home, "bin", "java"))
	if err != nil || info.IsDir() {
		return false
	}
	// The macOS /usr/bin/java stub lives at /usr — reject /usr as JAVA_HOME.
	if home == "/usr" || home == "/usr/" {
		return false
	}
	// A real JDK has a "release" file or "lib" directory.
	if _, err := os.Stat(filepath.Join(home, "release")); err == nil {
		return true
	}
	if info, err := os.Stat(filepath.Join(home, "lib")); err == nil && info.IsDir() {
		return true
	}
	return false
}

// findJDKHome searches well-known locations for a JDK.
func findJDKHome() string {
	homeDir, _ := os.UserHomeDir()

	candidates := []string{}

	// sdkman (Homebrew or ~/.sdkman)
	for _, sdkBase := range []string{
		"/opt/homebrew/opt/sdkman-cli/libexec",
		filepath.Join(homeDir, ".sdkman"),
	} {
		current := filepath.Join(sdkBase, "candidates", "java", "current")
		if isValidJDK(current) {
			return current
		}
	}

	// Gradle-provisioned JDKs (~/.gradle/jdks/*)
	gradleJDKs := filepath.Join(homeDir, ".gradle", "jdks")
	if entries, err := os.ReadDir(gradleJDKs); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			candidate := filepath.Join(gradleJDKs, e.Name())
			if isValidJDK(candidate) {
				candidates = append(candidates, candidate)
			}
		}
	}

	// Homebrew openjdk
	brewJDK := "/opt/homebrew/opt/openjdk/libexec/openjdk.jdk/Contents/Home"
	if isValidJDK(brewJDK) {
		candidates = append(candidates, brewJDK)
	}

	// java_home utility (macOS)
	if out, err := exec.Command("/usr/libexec/java_home").Output(); err == nil {
		jh := strings.TrimSpace(string(out))
		if isValidJDK(jh) {
			candidates = append(candidates, jh)
		}
	}

	// Resolve from java in PATH (follow real symlinks, skip macOS stub).
	if javaPath, err := exec.LookPath("java"); err == nil {
		if real, err := filepath.EvalSymlinks(javaPath); err == nil {
			home := filepath.Dir(filepath.Dir(real))
			if isValidJDK(home) {
				candidates = append(candidates, home)
			}
		}
	}

	if len(candidates) > 0 {
		return candidates[0]
	}
	return ""
}

// ensureGradleHome ensures GRADLE_USER_HOME is set in env. When running in a
// temp directory, Gradle may not inherit the default cache location, causing it
// to re-download all dependencies. This sets it to ~/.gradle if not already set.
func ensureGradleHome(env []string) []string {
	for _, e := range env {
		if len(e) > 16 && e[:16] == "GRADLE_USER_HOME" {
			return env // already set
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return env
	}
	gradleHome := filepath.Join(home, ".gradle")
	if info, err := os.Stat(gradleHome); err == nil && info.IsDir() {
		env = append(env, "GRADLE_USER_HOME="+gradleHome)
	}
	return env
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
