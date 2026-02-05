// SPDX-License-Identifier: MIT

// Package testutil provides testing utilities for lspls.
package testutil

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/tools/txtar"
)

// Case represents a parsed test case from a txtar archive.
type Case struct {
	// Name is the test case name (typically the filename without extension).
	Name string

	// Description is the first comment block before any files.
	Description string

	// Flags contains any flags parsed from "Flags: ..." line in the description.
	Flags []string

	// Input is the contents of "input.json".
	Input []byte

	// Want maps relative paths (e.g., "protocol.go") to expected content.
	Want map[string][]byte
}

// ParseCase parses a txtar archive into a test Case.
// The archive should contain:
//   - A description comment (text before first file)
//   - An "input.json" file with the LSP model JSON
//   - One or more "want/<filename>" files with expected output
//
// The description may contain a "Flags: flag1, flag2" line to pass flags
// to the generator.
func ParseCase(name string, ar *txtar.Archive) (*Case, error) {
	c := &Case{
		Name:        name,
		Description: string(ar.Comment),
		Want:        make(map[string][]byte),
	}

	// Parse flags from description
	c.parseFlags()

	// Process files
	for _, f := range ar.Files {
		switch {
		case f.Name == "input.json":
			c.Input = f.Data
		case strings.HasPrefix(f.Name, "want/"):
			relPath := strings.TrimPrefix(f.Name, "want/")
			c.Want[relPath] = f.Data
		default:
			return nil, fmt.Errorf("unexpected file in archive: %q (expected input.json or want/*)", f.Name)
		}
	}

	if c.Input == nil {
		return nil, fmt.Errorf("missing input.json in archive")
	}

	if len(c.Want) == 0 {
		return nil, fmt.Errorf("missing want/* files in archive")
	}

	return c, nil
}

// parseFlags extracts flags from "Flags: ..." line in the description.
func (c *Case) parseFlags() {
	lines := strings.Split(c.Description, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Flags:") {
			flagStr := strings.TrimPrefix(line, "Flags:")
			flagStr = strings.TrimSpace(flagStr)
			if flagStr != "" {
				for _, f := range strings.Split(flagStr, ",") {
					f = strings.TrimSpace(f)
					if f != "" {
						c.Flags = append(c.Flags, f)
					}
				}
			}
			break
		}
	}
}

// GenerateFunc is a function that generates output from input JSON.
// It returns a map of filename to content.
type GenerateFunc func(input []byte, flags []string) (map[string][]byte, error)

// Run executes the test case using the provided generate function.
// It compares generated output against expected output and reports differences.
func (c *Case) Run(t *testing.T, generate GenerateFunc) {
	t.Helper()

	got, err := generate(c.Input, c.Flags)
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	// Check for missing expected files
	for wantFile := range c.Want {
		if _, ok := got[wantFile]; !ok {
			t.Errorf("missing output file: %q", wantFile)
		}
	}

	// Check for unexpected files
	for gotFile := range got {
		if _, ok := c.Want[gotFile]; !ok {
			t.Errorf("unexpected output file: %q", gotFile)
		}
	}

	// Compare contents
	for wantFile, wantContent := range c.Want {
		gotContent, ok := got[wantFile]
		if !ok {
			continue // Already reported as missing
		}

		// Normalize line endings and trailing whitespace
		wantNorm := normalizeContent(wantContent)
		gotNorm := normalizeContent(gotContent)

		if diff := cmp.Diff(wantNorm, gotNorm); diff != "" {
			t.Errorf("file %q mismatch (-want +got):\n%s", wantFile, diff)
		}
	}
}

// normalizeContent normalizes content for comparison:
// - Trims trailing whitespace from each line
// - Ensures consistent line endings
// - Trims trailing newlines
func normalizeContent(content []byte) string {
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t\r")
	}
	result := strings.Join(lines, "\n")
	return strings.TrimRight(result, "\n")
}

// UpdateArchive updates a txtar archive with new generated content.
// Used for golden file updates with -update flag.
func UpdateArchive(ar *txtar.Archive, got map[string][]byte) *txtar.Archive {
	// Keep comment and input.json
	result := &txtar.Archive{
		Comment: ar.Comment,
	}

	for _, f := range ar.Files {
		if f.Name == "input.json" {
			result.Files = append(result.Files, f)
			break
		}
	}

	// Add want/* files in sorted order for determinism
	var wantFiles []string
	for name := range got {
		wantFiles = append(wantFiles, name)
	}
	sort.Strings(wantFiles)

	for _, name := range wantFiles {
		content := got[name]
		// Ensure trailing newline
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

// FormatArchive formats an archive to bytes.
func FormatArchive(ar *txtar.Archive) []byte {
	return txtar.Format(ar)
}

// LoadTestCases loads all txtar test cases from a directory.
func LoadTestCases(t *testing.T, dir string) []*Case {
	t.Helper()

	pattern := filepath.Join(dir, "*.txtar")
	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob %q: %v", pattern, err)
	}

	if len(files) == 0 {
		t.Fatalf("no txtar files found in %q", dir)
	}

	var cases []*Case
	for _, file := range files {
		ar, err := txtar.ParseFile(file)
		if err != nil {
			t.Fatalf("parse %q: %v", file, err)
		}

		name := strings.TrimSuffix(filepath.Base(file), ".txtar")
		c, err := ParseCase(name, ar)
		if err != nil {
			t.Fatalf("parse case %q: %v", name, err)
		}

		cases = append(cases, c)
	}

	// Sort by name for determinism
	sort.Slice(cases, func(i, j int) bool {
		return cases[i].Name < cases[j].Name
	})

	return cases
}

// StripHeader removes the "Code generated by lspls" header from generated code.
// This allows tests to compare just the meaningful code.
func StripHeader(content []byte) []byte {
	lines := bytes.Split(content, []byte("\n"))
	var result [][]byte
	inHeader := true

	for _, line := range lines {
		lineStr := string(line)
		// Skip header lines (comments at the start)
		if inHeader {
			if strings.HasPrefix(lineStr, "//") || lineStr == "" {
				continue
			}
			inHeader = false
		}
		result = append(result, line)
	}

	return bytes.Join(result, []byte("\n"))
}
