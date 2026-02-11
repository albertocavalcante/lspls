// SPDX-License-Identifier: MIT
//
// Copyright 2026 Alberto Cavalcante. All rights reserved.
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

package proto

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albertocavalcante/lspls/internal/testutil"
	"github.com/albertocavalcante/lspls/model"
	"golang.org/x/tools/txtar"
)

func TestConvertBaseType(t *testing.T) {
	tests := []struct {
		name    string
		lspType string
		want    string
		wantErr bool
	}{
		{name: "string", lspType: "string", want: "string"},
		{name: "integer", lspType: "integer", want: "int32"},
		{name: "uinteger", lspType: "uinteger", want: "uint32"},
		{name: "decimal", lspType: "decimal", want: "double"},
		{name: "boolean", lspType: "boolean", want: "bool"},
		{name: "null", lspType: "null", want: "", wantErr: true},
		{name: "DocumentUri", lspType: "DocumentUri", want: "string"},
		{name: "URI", lspType: "URI", want: "string"},
		{name: "unknown", lspType: "unknown", want: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertBaseType(tt.lspType)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertBaseType(%q) error = %v, wantErr %v", tt.lspType, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("convertBaseType(%q) = %q, want %q", tt.lspType, got, tt.want)
			}
		})
	}
}

func TestToProtoMessageName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{name: "Position", want: "Position"},
		{name: "TextDocumentIdentifier", want: "TextDocumentIdentifier"},
		{name: "InlayHintKind", want: "InlayHintKind"},
		{name: "$foo", want: "Foo"}, // strip leading $ and capitalize
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toProtoMessageName(tt.name)
			if got != tt.want {
				t.Errorf("toProtoMessageName(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestToProtoFieldName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{name: "line", want: "line"},
		{name: "character", want: "character"},
		{name: "textDocument", want: "text_document"},
		{name: "documentUri", want: "document_uri"},
		{name: "URI", want: "uri"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toProtoFieldName(tt.name)
			if got != tt.want {
				t.Errorf("toProtoFieldName(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestGenerateMessage(t *testing.T) {
	g := &Codegen{
		config: Config{PackageName: "lsp"},
	}

	structure := &model.Structure{
		Name:          "Position",
		Documentation: "Position in a text document.",
		Properties: []model.Property{
			{Name: "line", Type: &model.Type{Kind: "base", Name: "uinteger"}},
			{Name: "character", Type: &model.Type{Kind: "base", Name: "uinteger"}},
		},
	}

	got := g.generateMessage(structure)

	// Verify key parts
	if !strings.Contains(got, "message Position") {
		t.Errorf("expected 'message Position' in output:\n%s", got)
	}
	if !strings.Contains(got, "uint32 line = 1") {
		t.Errorf("expected 'uint32 line = 1' in output:\n%s", got)
	}
	if !strings.Contains(got, "uint32 character = 2") {
		t.Errorf("expected 'uint32 character = 2' in output:\n%s", got)
	}
	if !strings.Contains(got, "// Position in a text document.") {
		t.Errorf("expected documentation comment in output:\n%s", got)
	}
}

func TestGenerateEnum(t *testing.T) {
	g := &Codegen{
		config: Config{PackageName: "lsp"},
	}

	enum := &model.Enumeration{
		Name:          "InlayHintKind",
		Documentation: "Inlay hint kinds.",
		Type:          &model.Type{Kind: "base", Name: "uinteger"},
		Values: []model.EnumValue{
			{Name: "Type", Value: float64(1)},
			{Name: "Parameter", Value: float64(2)},
		},
	}

	got := g.generateEnum(enum)

	// Verify key parts
	if !strings.Contains(got, "enum InlayHintKind") {
		t.Errorf("expected 'enum InlayHintKind' in output:\n%s", got)
	}
	// Proto3 enums must have 0 as first value
	if !strings.Contains(got, "INLAY_HINT_KIND_UNSPECIFIED = 0") {
		t.Errorf("expected UNSPECIFIED = 0 in output:\n%s", got)
	}
	if !strings.Contains(got, "INLAY_HINT_KIND_TYPE = 1") {
		t.Errorf("expected TYPE = 1 in output:\n%s", got)
	}
	if !strings.Contains(got, "INLAY_HINT_KIND_PARAMETER = 2") {
		t.Errorf("expected PARAMETER = 2 in output:\n%s", got)
	}
}

func TestGenerateEnumString(t *testing.T) {
	g := &Codegen{
		config: Config{PackageName: "lsp"},
	}

	enum := &model.Enumeration{
		Name: "TokenFormat",
		Type: &model.Type{Kind: "base", Name: "string"},
		Values: []model.EnumValue{
			{Name: "Relative", Value: "relative"},
		},
	}

	got := g.generateEnum(enum)

	// String enums need special handling - proto3 enums are always int
	// We generate a message with string constant instead
	if !strings.Contains(got, "TOKEN_FORMAT") || !strings.Contains(got, "RELATIVE") {
		t.Errorf("expected string enum handling in output:\n%s", got)
	}
}

// TestCodegen runs txtar-based integration tests.
func TestCodegen(t *testing.T) {
	testdataDir := filepath.Join("testdata")

	pattern := filepath.Join(testdataDir, "*.txtar")
	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob %q: %v", pattern, err)
	}

	if len(files) == 0 {
		t.Skip("no txtar files found in testdata/")
	}

	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".txtar")
		t.Run(name, func(t *testing.T) {
			ar, err := txtar.ParseFile(file)
			if err != nil {
				t.Fatalf("parse txtar: %v", err)
			}

			tc, err := testutil.ParseCase(name, ar)
			if err != nil {
				t.Fatalf("parse case: %v", err)
			}

			generate := func(input []byte, flags []string) (map[string][]byte, error) {
				return runCodegen(input, flags)
			}

			if *update {
				got, err := generate(tc.Input, tc.Flags)
				if err != nil {
					t.Fatalf("generate: %v", err)
				}

				updated := testutil.UpdateArchive(ar, got)
				content := testutil.FormatArchive(updated)

				if err := os.WriteFile(file, content, 0o644); err != nil {
					t.Fatalf("write updated file: %v", err)
				}
				t.Logf("updated %s", file)
				return
			}

			tc.Run(t, generate)
		})
	}
}

var update = flag.Bool("update", false, "update golden files")

// runCodegen generates proto from input JSON.
func runCodegen(input []byte, flags []string) (map[string][]byte, error) {
	var m model.Model
	if err := json.Unmarshal(input, &m); err != nil {
		return nil, err
	}

	cfg := Config{
		PackageName: "lsp",
	}

	// Parse flags
	for _, f := range flags {
		if pkgName, ok := strings.CutPrefix(f, "package="); ok {
			cfg.PackageName = pkgName
		}
		if typesStr, ok := strings.CutPrefix(f, "types="); ok {
			for _, t := range strings.Split(typesStr, ";") {
				t = strings.TrimSpace(t)
				if t != "" {
					cfg.Types = append(cfg.Types, t)
				}
			}
		}
		if val, ok := strings.CutPrefix(f, "resolve-deps="); ok {
			cfg.ResolveDeps = val == "true"
		}
	}

	gen := New(&m, cfg)
	out, err := gen.Generate()
	if err != nil {
		return nil, err
	}

	// Strip variable header for comparison
	proto := stripGeneratedHeader(out.Proto)

	return map[string][]byte{"protocol.proto": proto}, nil
}

// stripGeneratedHeader removes variable parts of the header.
func stripGeneratedHeader(content []byte) []byte {
	lines := strings.Split(string(content), "\n")
	var result []string
	inHeader := true

	for _, line := range lines {
		if strings.HasPrefix(line, "// Code generated by lspls") {
			result = append(result, line)
			continue
		}
		if inHeader && strings.HasPrefix(line, "// ") {
			continue
		}
		if inHeader && !strings.HasPrefix(line, "//") {
			inHeader = false
		}
		result = append(result, line)
	}

	return []byte(strings.Join(result, "\n"))
}
