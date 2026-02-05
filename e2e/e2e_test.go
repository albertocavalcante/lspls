// SPDX-License-Identifier: MIT

// Package e2e provides end-to-end tests for the lspls CLI.
package e2e

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/tools/txtar"
)

var (
	binary string                                              // path to built lspls binary
	update = flag.Bool("update", false, "update golden files")
)

func TestMain(m *testing.M) {
	flag.Parse()

	// Build the lspls binary to a temp location.
	tmpDir, err := os.MkdirTemp("", "lspls-e2e-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	binary = filepath.Join(tmpDir, "lspls")
	if err := buildBinary(binary); err != nil {
		fmt.Fprintf(os.Stderr, "failed to build binary: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// buildBinary builds the lspls binary to the specified path.
func buildBinary(outputPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Build from the cmd/lspls directory.
	cmd := exec.CommandContext(ctx, "go", "build", "-o", outputPath, "./cmd/lspls")

	// Set working directory to the module root.
	moduleRoot, err := findModuleRoot()
	if err != nil {
		return fmt.Errorf("find module root: %w", err)
	}
	cmd.Dir = moduleRoot

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build: %w: %s", err, stderr.String())
	}

	return nil
}

// findModuleRoot finds the root of the Go module by looking for go.mod.
func findModuleRoot() (string, error) {
	// Start from the current working directory and walk up.
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// The e2e tests are in /path/to/lspls/e2e/, so module root is the parent.
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

func TestE2E(t *testing.T) {
	testdataDir := filepath.Join("testdata")

	pattern := filepath.Join(testdataDir, "*.txtar")
	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob %q: %v", pattern, err)
	}

	if len(files) == 0 {
		t.Fatalf("no txtar files found in %q", testdataDir)
	}

	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".txtar")
		t.Run(name, func(t *testing.T) {
			runTestCase(t, file, name)
		})
	}
}

// runTestCase executes a single e2e test case.
func runTestCase(t *testing.T, file, name string) {
	t.Helper()

	ar, err := txtar.ParseFile(file)
	if err != nil {
		t.Fatalf("parse txtar: %v", err)
	}

	tc, err := parseE2ECase(name, ar)
	if err != nil {
		t.Fatalf("parse case: %v", err)
	}

	// Create temp directory for input files.
	tmpDir, err := os.MkdirTemp("", "lspls-e2e-case-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	// Write input.json to temp directory.
	inputPath := filepath.Join(tmpDir, "input.json")
	if err := os.WriteFile(inputPath, tc.input, 0o644); err != nil {
		t.Fatalf("write input.json: %v", err)
	}

	// Build command arguments.
	args := []string{"--spec", inputPath, "--dry-run"}
	args = append(args, tc.flags...)

	// Execute the CLI.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		t.Logf("command: %s %s", binary, strings.Join(args, " "))
		t.Logf("stderr: %s", stderr.String())
		t.Fatalf("command failed: %v", err)
	}

	// The output is the stdout content.
	got := map[string][]byte{
		"stdout": stdout.Bytes(),
	}

	if *update {
		updated := updateE2EArchive(ar, got)
		content := txtar.Format(updated)

		if err := os.WriteFile(file, content, 0o644); err != nil {
			t.Fatalf("write updated file: %v", err)
		}
		t.Logf("updated %s", file)
		return
	}

	// Compare output.
	compareOutput(t, tc.want, got)
}

// e2eCase represents a parsed e2e test case.
type e2eCase struct {
	name        string
	description string
	flags       []string
	input       []byte
	want        map[string][]byte
}

// parseE2ECase parses a txtar archive into an e2e test case.
func parseE2ECase(name string, ar *txtar.Archive) (*e2eCase, error) {
	c := &e2eCase{
		name:        name,
		description: string(ar.Comment),
		want:        make(map[string][]byte),
	}

	// Parse flags from description.
	c.parseFlags()

	// Process files.
	for _, f := range ar.Files {
		switch {
		case f.Name == "input.json":
			c.input = f.Data
		case strings.HasPrefix(f.Name, "want/"):
			relPath := strings.TrimPrefix(f.Name, "want/")
			c.want[relPath] = f.Data
		default:
			return nil, fmt.Errorf("unexpected file in archive: %q (expected input.json or want/*)", f.Name)
		}
	}

	if c.input == nil {
		return nil, fmt.Errorf("missing input.json in archive")
	}

	if len(c.want) == 0 {
		return nil, fmt.Errorf("missing want/* files in archive")
	}

	return c, nil
}

// parseFlags extracts flags from "Flags: ..." line in the description.
// Flags are space-separated (not comma-separated) to match CLI conventions.
func (c *e2eCase) parseFlags() {
	lines := strings.Split(c.description, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Flags:") {
			flagStr := strings.TrimPrefix(line, "Flags:")
			flagStr = strings.TrimSpace(flagStr)
			if flagStr != "" {
				// Split by whitespace to handle flags like "-t Position,Range"
				c.flags = strings.Fields(flagStr)
			}
			break
		}
	}
}

// compareOutput compares expected and actual output.
func compareOutput(t *testing.T, want, got map[string][]byte) {
	t.Helper()

	// Check for missing expected files.
	for wantFile := range want {
		if _, ok := got[wantFile]; !ok {
			t.Errorf("missing output file: %q", wantFile)
		}
	}

	// Check for unexpected files.
	for gotFile := range got {
		if _, ok := want[gotFile]; !ok {
			t.Errorf("unexpected output file: %q", gotFile)
		}
	}

	// Compare contents.
	for wantFile, wantContent := range want {
		gotContent, ok := got[wantFile]
		if !ok {
			continue // Already reported as missing.
		}

		// Strip the variable header from both for comparison.
		wantNorm := normalizeOutput(wantContent)
		gotNorm := normalizeOutput(gotContent)

		if diff := cmp.Diff(wantNorm, gotNorm); diff != "" {
			t.Errorf("file %q mismatch (-want +got):\n%s", wantFile, diff)
		}
	}
}

// normalizeOutput normalizes output for comparison.
// It strips the variable parts of the generated header (Source, Ref, Commit, LSP Version).
func normalizeOutput(content []byte) string {
	lines := strings.Split(string(content), "\n")
	var result []string
	foundGenerated := false

	for _, line := range lines {
		// Keep the "Code generated" line.
		if strings.HasPrefix(line, "// Code generated by lspls") {
			result = append(result, line)
			foundGenerated = true
			continue
		}
		// Skip other header comments (Source, Ref, Commit, LSP Version).
		if foundGenerated && strings.HasPrefix(line, "// ") {
			continue
		}
		// Once we hit a non-comment line after the header, include everything.
		if foundGenerated || !strings.HasPrefix(line, "//") {
			result = append(result, line)
			foundGenerated = true
		}
	}

	// Trim trailing whitespace from each line and trailing newlines.
	for i, line := range result {
		result[i] = strings.TrimRight(line, " \t\r")
	}
	return strings.TrimRight(strings.Join(result, "\n"), "\n")
}

// updateE2EArchive updates a txtar archive with new generated content.
func updateE2EArchive(ar *txtar.Archive, got map[string][]byte) *txtar.Archive {
	result := &txtar.Archive{
		Comment: ar.Comment,
	}

	// Keep input.json.
	for _, f := range ar.Files {
		if f.Name == "input.json" {
			result.Files = append(result.Files, f)
			break
		}
	}

	// Add want/* files in sorted order.
	var wantFiles []string
	for name := range got {
		wantFiles = append(wantFiles, name)
	}
	sort.Strings(wantFiles)

	for _, name := range wantFiles {
		content := got[name]
		// Strip variable header for storage.
		content = []byte(normalizeForStorage(content))
		// Ensure trailing newline.
		if len(content) > 0 && content[len(content)-1] != '\n' {
			content = append(content, '\n')
		}
		result.Files = append(result.Files, txtar.File{
			Name: "want/" + name,
			Data: content,
		})
	}

	return result
}

// normalizeForStorage normalizes output for storage in golden files.
// Similar to normalizeOutput but preserves the basic header structure.
func normalizeForStorage(content []byte) string {
	lines := strings.Split(string(content), "\n")
	var result []string
	foundGenerated := false

	for _, line := range lines {
		// Keep the "Code generated" line.
		if strings.HasPrefix(line, "// Code generated by lspls") {
			result = append(result, line)
			foundGenerated = true
			continue
		}
		// Skip other header comments (Source, Ref, Commit, LSP Version).
		if foundGenerated && strings.HasPrefix(line, "// ") {
			continue
		}
		// Once we hit a non-comment line after the header, include everything.
		if foundGenerated || !strings.HasPrefix(line, "//") {
			result = append(result, line)
			foundGenerated = true
		}
	}

	// Trim trailing whitespace from each line.
	for i, line := range result {
		result[i] = strings.TrimRight(line, " \t\r")
	}
	return strings.Join(result, "\n")
}
